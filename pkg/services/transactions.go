package services

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/store"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type transactionService struct {
	*mgo.Collection
}

func GetTransactions() *transactionService {
	return &transactionService{}
}

func (s *transactionService) GetAmount(user *store.UserMgo) (amount float64) {
	c := store.GetCollection("transactions")

	pipe := c.Pipe([]bson.M{
		{
			"$match": bson.M{"user": user.ID},
		}, {
			"$group": bson.M{
				"_id":      1,
				"earnings": bson.M{"$sum": "$amount"},
			},
		}, {
			"$project": bson.M{
				"earnings": 1,
			},
		},
	})

	result := make(map[string]float64)

	if err := pipe.One(&result); err != nil {
		logger.Get().Errorf("error getting earnings: %v", err)
		return 0
	}

	logger.Get().Debug(result)
	value, exist := result["earnings"]

	if exist {
		return value
	}

	return
}

func (s *transactionService) GetTransactions(user *store.UserMgo, from, to time.Time) (transactions []*store.TransactionDto) {
	c := store.GetCollection("transactions")

	pipe := c.Pipe([]bson.M{
		{
			"$match": bson.M{
				"user": user.ID,
				"time": bson.M{
					"$gte": from,
					"$lte": to,
				},
			},
		},
		{
			"$sort": bson.M{
				"time": -1,
			},
		},
		{
			"$project": bson.M{
				"_id":       1,
				"user":      1,
				"amount":    1,
				"lesson":    1,
				"details":   1,
				"reference": 1,
				"time":      1,
				"state":     1,
			},
		},
	})

	var transactionMgos []*store.TransactionMgo
	if err := pipe.All(&transactionMgos); err != nil {
		logger.Get().Errorf("error GetTransactions(): %v", err)
	}

	transactions = make([]*store.TransactionDto, 0)
	for _, m := range transactionMgos {
		transactions = append(transactions, m.Dto())
	}

	return
}

func (s *transactionService) GetCreditSummaryTransactions(from, to time.Time) (transactions []*store.TransactionDto) {
	c := store.GetCollection("transactions")

	pipe := c.Pipe([]bson.M{
		{
			"$match": bson.M{
				"status": "credit",
				"time": bson.M{
					"$gte": from,
					"$lte": to,
				},
			},
		},
		{
			"$sort": bson.M{
				"time": -1,
			},
		},
		{
			"$project": bson.M{
				"_id":       1,
				"user":      1,
				"amount":    1,
				"lesson":    1,
				"details":   1,
				"reference": 1,
				"time":      1,
				"state":     1,
			},
		},
	})

	var transactionMgos []*store.TransactionMgo
	if err := pipe.All(&transactionMgos); err != nil {
		logger.Get().Errorf("error GetTransactions(): %v", err)
	}

	transactions = make([]*store.TransactionDto, 0)
	for _, m := range transactionMgos {
		transactions = append(transactions, m.Dto())
	}

	return
}

func (s *transactionService) GetTransactionsPaged(user *store.UserMgo, from time.Time, to time.Time, page int, limit int) (count int, transactions []*store.TransactionDto) {
	collection := store.GetCollection("transactions")
	query := collection.Find(bson.M{
		"user": user.ID,
		"time": bson.M{
			"$gte": from,
			"$lte": to,
		},
	}).Sort("-time")
	count, _ = query.Count()
	var transactionsMgo []*store.TransactionMgo
	if err := query.Skip((page - 1) * limit).Limit(limit).All(&transactionsMgo); err != nil {
		logger.Get().Errorf("error GetTransactions(): %v", err)
		return 0, nil
	}

	transactions = make([]*store.TransactionDto, 0)
	for _, m := range transactionsMgo {
		transactions = append(transactions, m.Dto())
	}
	return count, transactions
}

func (s *transactionService) New(t *store.TransactionMgo) (*store.TransactionMgo, error) {
	if !t.ID.Valid() {
		t.ID = bson.NewObjectId()
	}

	if t.Amount == 0 {
		return nil, errors.New("amount can't be zero")
	}

	if t.Details == "" {
		return nil, errors.New("details or reference can't be blank")
	}

	now := time.Now()
	if t.Time.IsZero() {
		t.Time = now
	}

	collection := store.GetCollection("transactions")
	if t.Reference == "" {
		c, err := collection.Count()
		if err != nil {
			return t, errors.Wrap(err, "couldn't count transactions collection")
		}
		t.Reference = fmt.Sprintf("INV-%d-%d%d%d", c+1, now.Year(), now.Month(), now.Day())
	}

	return t, collection.Insert(t)
}
