package payments

import (
	"context"
	"fmt"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/routes/auth"
	"gitlab.com/learnt/api/pkg/routes/register"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/services/stripe"
	"gitlab.com/learnt/api/pkg/store"
	"gopkg.in/mgo.v2/bson"
)

func me(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}

	c.JSON(http.StatusOK, user.Payments)
}

type errorResponse struct {
	Error   bool   `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
	Data    string `json:"data,omitempty"`
}

type balanceResponse struct {
	errorResponse
	Balance int64 `json:"balance"`
}

func balance(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}

	res := balanceResponse{}

	p := services.GetPayments()
	b, err := p.GetBalance(user)
	if err != nil {
		res.Error = true
		res.Message = fmt.Sprintf("couldn't get user balance: %s", err)

		c.JSON(http.StatusBadRequest, res)
		return
	}

	res.Balance = b
	c.JSON(http.StatusOK, res)
}

type cardResponse struct {
	errorResponse
	Card *stripe.Card `json:"card,omitempty"`
}

func deleteCard(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}

	res := cardResponse{}
	p := services.GetPayments()
	var err error
	res.Card, err = p.DeleteCard(user, c.Param("id"))
	if err != nil {
		res.Error = true
		res.Message = err.Error()
		c.JSON(http.StatusBadRequest, res)
		return
	}

	c.JSON(http.StatusOK, res)
}

func deleteBankAccount(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}

	res := cardResponse{}
	p := services.GetPayments()
	var err error
	err = p.DeleteBankAccount(user, c.Param("id"))
	if err != nil {
		res.Error = true
		res.Message = err.Error()
		c.JSON(http.StatusBadRequest, res)
		return
	}

	c.JSON(http.StatusOK, res)
}

func setDefaultCard(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}

	res := cardResponse{}
	p := services.GetPayments()
	var err error
	res.Card, err = p.SetDefaultCreditCard(user, c.Param("id"))
	if err != nil {
		res.Error = true
		res.Message = err.Error()
		c.JSON(http.StatusBadRequest, res)
		return
	}

	c.JSON(http.StatusOK, res)
}

func ensureConnectAccount(c *gin.Context) {
	res := errorResponse{}
	user, ok := store.GetUser(c)
	if !ok {
		res.Error = true
		res.Message = "Unauthorized"
		c.JSON(http.StatusUnauthorized, res)
		return
	}

	if user.HasRole(store.RoleStudent) {
		res.Error = true
		res.Message = "Students cannot have connect accounts"
		c.JSON(http.StatusBadRequest, res)
		return
	}

	req := struct {
		SocialSecurityNumber string `json:"social_security_number"`
		CompanyName          string `json:"company_name"`
		CompanyEIN           string `json:"company_ein"`
	}{}

	if err := c.ShouldBindJSON(&req); err != nil {
		res.Error = true
		res.Message = "error binding json to struct"
		err = errors.Wrap(err, res.Message)
		res.Data = err.Error()
		c.JSON(http.StatusBadRequest, res)
		return
	}

	if req.SocialSecurityNumber == "" {
		res.Error = true
		res.Message = "At least the last 4 of a social security number is required"
		c.JSON(http.StatusBadRequest, res)
		return
	}

	if user.Payments != nil && user.Payments.ConnectID != "" {
		res.Error = true
		res.Message = "Account already has a connect account"
		c.JSON(http.StatusBadRequest, res)
		return
	}

	if err := register.CreateStripeConnectAccount(c, user, req.SocialSecurityNumber, req.CompanyName, req.CompanyEIN); err != nil {
		res.Error = true
		res.Message = "Can't create payment account"
		res.Data = errors.Wrap(err, res.Message).Error()
		logger.GetCtx(c).Errorf("failed to create stripe account for user %v: %v", user, err)
		c.JSON(http.StatusBadRequest, res)
		return
	}

	res.Message = "Stripe connect account created"
	res.Data = user.Payments.ConnectID
	c.JSON(http.StatusCreated, res)
}

type creditRequest struct {
	Amount float64 `json:"amount"`
	Reason string  `json:"reason"`
	Notes  string  `json:"notes"`
}

func addCredit(c *gin.Context) {

	auth.IsAdminMiddleware(c)

	userId := c.Param("id")
	if !bson.IsObjectIdHex(userId) {
		c.JSON(http.StatusBadRequest, errorResponse{Error: true, Message: "invalid userId"})
		return
	}

	user, exist := services.NewUsers().ByID(bson.ObjectIdHex(userId))
	if !exist {
		c.Status(http.StatusNotFound)
		return
	}

	creditReq := creditRequest{}

	if err := c.BindJSON(&creditReq); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: true, Message: "invalid parameter"})
		return
	}

	notes := creditReq.Notes

	if notes != "" {
		notes = "Note: " + notes
	}

	creditParams := services.CreditParams{}
	creditParams.Notes = fmt.Sprintf("Granted %.2f in credits from Learnt Admin. %s", creditReq.Amount, notes)
	creditParams.Reason = "credit"
	creditParams.Amount = int64(math.Ceil(creditReq.Amount * 100))

	p := services.GetPayments()
	if err := p.AddCredits(user, creditParams); err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: true, Message: "there is an error adding credits"})
	}
}

// test handlers

func cards(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}

	p := services.GetPayments()
	cards, err := p.GetCards(user)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, cards)
}

// Setup adds the payment routes to the gin router
func Setup(ctx context.Context, g *gin.RouterGroup) {
	services.GetPayments().Init(ctx)

	g.GET("", me)
	g.GET("balance", balance)
	g.DELETE("cards/:id", deleteCard)
	g.DELETE("bankaccount/:id", deleteBankAccount)
	g.PUT("default/:id", setDefaultCard)
	g.POST("ensureconnect", ensureConnectAccount)
	g.PUT("add-credit/:id", addCredit)
}
