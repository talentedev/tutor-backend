package services

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	jose "github.com/dvsekhvalnov/jose2go"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/store"

	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type users struct {
	*mgo.Collection
}

// NewUsers returns an instance of the uses object that
// wraps the uses store collection.
func NewUsers() *users {
	return &users{
		store.GetCollection("users"),
	}
}

// ByUsername searches for a user by its username.
func (u *users) ByUsername(username string) (user *store.UserMgo, exist bool) {
	exist = u.Find(bson.M{"username": username}).One(&user) == nil
	return
}

// ByID searches for a user by its ID.
func (u *users) ByID(id bson.ObjectId) (user *store.UserMgo, exist bool) {
	exist = u.FindId(id).One(&user) == nil
	return
}

// ByID searches for a user by its ID.
func (u *users) ByRole(role store.Role) (user *store.UserMgo, exist bool) {
	exist = u.FindId(bson.M{"role": role}).One(&user) == nil
	return
}

// ByEmailVerified searches for a user by its verified email.
func (u *users) ByEmailVerified(email string) (user *store.UserMgo, exist bool) {
	exist = u.Find(bson.M{"emails.email": email, "emails.verified": true}).One(&user) == nil
	return
}

// ByEmail searches for a user by its email.
func (u *users) ByEmail(email string) (user *store.UserMgo, exist bool) {
	exist = u.Find(bson.M{"emails.email": email}).One(&user) == nil
	return
}

// BySocialNetwork searches for a user by its network and sub.
func (u *users) BySocialNetwork(network, sub string) (user *store.UserMgo, exist bool) {

	exist = u.Find(bson.M{
		"$and": []bson.M{
			{"social_networks.sub": sub},
			{"social_networks.network": network},
		},
	}).One(&user) == nil
	return
}

// ByIDs searches for users with the specified IDs.
func (u *users) ByIDs(id []bson.ObjectId) (users []*store.UserMgo) {
	u.Find(bson.M{"_id": bson.M{"$in": id}}).All(&users)
	return
}

// ByReferralCode searches for a user by its referral code.
func (u *users) ByReferralCode(r string) (user *store.UserMgo, exist bool) {
	exist = u.Find(bson.M{"refer.referral_code": r}).One(&user) == nil
	return
}

// All returns all users.
func (u *users) All() (users []*store.UserDto) {
	u.Find(bson.M{}).All(&users)
	return
}

// RequiresVerification returns all users that need to be verified.
func (u *users) RequiresVerification() (users []*store.UserDto, err error) {
	query := u.Find(bson.M{
		"approval": store.ApprovalStatusApproved,
		"tutoring": bson.M{"$exists": true},
		"$or": []bson.M{
			{"tutoring.degrees.verified": false},
			{"tutoring.subjects.verified": false},
		},
	})

	users = make([]*store.UserDto, 0)
	err = query.Sort("-registered_date").All(&users)

	return
}

// Count returns the approved users with the specified role.
func (u *users) Count(role store.Role) int {
	q := u.Find(bson.M{
		"approval": store.ApprovalStatusApproved,
		"role":     bson.M{"$eq": role},
	})

	n, _ := q.Count()

	return n
}

// PendingTutors returns all pending tutors.
func (u *users) PendingTutors() (users []*store.UserDto, err error) {
	query := u.Find(bson.M{
		"approval": store.ApprovalStatusNew,
		"tutoring": bson.M{"$exists": true},
	})

	users = make([]*store.UserDto, 0)
	err = query.Sort("-registered_date").All(&users)

	return
}

// Approve activates a user.
func (u *users) Approve(admin *store.UserMgo, user bson.ObjectId) error {
	return u.UpdateId(user, bson.M{
		"$set": bson.M{
			"approval":            store.ApprovalStatusApproved,
			"approval_updated_at": time.Now().UTC(),
		},
	})
}

func (u *users) GetLessons(user *store.UserMgo) []*store.LessonDto {
	var l []store.LessonMgo
	var condition = []bson.M{
		{"tutor": user.ID},
		{"students": user.ID},
	}

	query := bson.M{
		"$or": condition,
	}

	store.GetCollection("lessons").Find(query).All(&l)

	return store.LessonsToDTO(l)
}

// Reject rejects a user.
func (u *users) Reject(admin *store.UserMgo, user bson.ObjectId, reason string) error {
	rejectType := store.RejectType
	rejectNote := store.UserNote{
		ID:        bson.NewObjectId(),
		Type:      &rejectType,
		Note:      reason,
		CreatedAt: time.Now().UTC(),
		UpdatedBy: store.NoteUpdatedBy{
			ID:   admin.ID,
			Name: admin.GetName(),
		},
	}

	return u.UpdateId(user, bson.M{
		"$set": bson.M{
			"approval":            store.ApprovalStatusRejected,
			"approval_updated_at": time.Now().UTC(),
		},
		"$push": bson.M{"notes": rejectNote},
	})
}

// Verify sets the verified flag for a degree or a subject.
func (u *users) Verify(user bson.ObjectId, what string, id bson.ObjectId) (err error) {
	if !user.Valid() {
		return errors.New("invalid object id for user")
	}

	if !id.Valid() {
		return errors.New("invalid object id for type")
	}

	pathObject := "tutoring." + what + "s"
	pathVerified := pathObject + ".$.verified"

	return u.Update(bson.M{
		"_id":      user,
		pathObject: bson.M{"$elemMatch": bson.M{"_id": id}},
	}, bson.M{
		"$set": bson.M{pathVerified: true},
	})
}

// ParseAuthenticationToken decodes a JOSE token, and tries to get the user who requested it
// and the token response.
func (u *users) ParseAuthenticationToken(accessToken string) (token *store.TokenResponse, user *store.UserMgo, err error) {
	userID, headers, err := jose.Decode(accessToken, []byte(config.GetConfig().GetString("security.token")))
	if err != nil {
		return nil, nil, err
	}

	eat := time.Unix(int64(headers["eat"].(float64)), 0)
	scope := headers["scope"].(string)

	if time.Now().After(eat) {
		return nil, nil, errors.New("token expired")
	}

	if !bson.IsObjectIdHex(userID) {
		return nil, nil, errors.New("token contains invalid user id")
	}

	if err := u.FindId(bson.ObjectIdHex(userID)).One(&user); err != nil {
		return nil, nil, errors.New("user not found")
	}

	token = &store.TokenResponse{
		AccessToken: accessToken,
		Expires:     eat.Unix(),
		ExpiresIn:   eat.Unix() - time.Now().Unix(),
		TokenType:   "Bearer",
		Scope:       scope,
	}

	return
}

// ByCandidateID searches for users with the given background check candidate ID.
func (u *users) ByCandidateID(candidateID string) (*store.UserMgo, error) {
	user := &store.UserMgo{}
	err := u.Find(bson.M{
		"$or": []bson.M{
			{"checkr_data.candidate_id": candidateID},
			{"bg_check_data.candidate_id": candidateID},
		},
	}).One(&user)
	if err != nil {
		return nil, err
	}
	return user, err
}

func (u *users) SetCheckrData(id bson.ObjectId, data *store.UserCheckrData) error {
	if data == nil {
		return fmt.Errorf("data can't be nil")
	}

	err := u.UpdateId(id, bson.M{"$set": bson.M{
		"checkr_data": data,
	}})
	if err != nil {
		return fmt.Errorf("error updating data in the database: %s", err)
	}

	return nil
}

func (u *users) SetBGCheckData(id bson.ObjectId, data *store.UserBGCheckData) error {
	if data == nil {
		return fmt.Errorf("data can't be nil")
	}

	err := u.UpdateId(id, bson.M{
		"$set": bson.M{
			"bg_check_data": data,
		},
	})
	if err != nil {
		return fmt.Errorf("error updating data in the database: %w", err)
	}

	return nil
}

func (u *users) GetStudents(page, limit int, q string) (users []*store.UserMgo, count int, err error) {
	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = 50
	}
	searchRegex := bson.RegEx{
		Pattern: regexp.QuoteMeta(q),
		Options: "i",
	}
	query := u.Find(bson.M{
		"role": store.RoleStudent,
		"$or": []bson.M{
			{"username": searchRegex},
			{"profile.first_name": searchRegex},
			{"profile.last_name": searchRegex},
		},
	})
	if count, err = query.Count(); err != nil {
		return nil, count, err
	}
	if err = query.Skip((page - 1) * limit).Limit(limit).Sort("-registered_date").All(&users); err != nil {
		return nil, count, err
	}
	return users, count, nil
}

// GetTutors returns approved tutors
func (u *users) GetTutors(page, limit int, q string, approvedOnly bool, subjectId string) (users []*store.UserMgo, count int, err error) {
	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = 50
	}
	searchRegex := bson.RegEx{
		Pattern: regexp.QuoteMeta(q),
		Options: "i",
	}

	approval := bson.M{
		"approval": bson.M{
			"$ne": store.ApprovalStatusRejected,
		},
	}
	if !approvedOnly {
		approval = bson.M{
			"approval": bson.M{"$exists": true},
		}
	}

	requiredFilters := []bson.M{
		{"role": store.RoleTutor},
		approval,
	}

	if subjectId != "" && bson.IsObjectIdHex(subjectId) {
		requiredFilters = append(requiredFilters, bson.M{"tutoring.subjects.subject": bson.ObjectIdHex(subjectId)})
	}

	query := u.Find(bson.M{
		"$and": requiredFilters,
		"$or": []bson.M{
			{"username": searchRegex},
			{"profile.first_name": searchRegex},
			{"profile.last_name": searchRegex},
		},
	})
	if count, err = query.Count(); err != nil {
		return nil, count, err
	}
	if err = query.Sort("approval").Limit(limit).Skip((page - 1) * limit).All(&users); err != nil {
		return nil, count, err
	}
	return
}

func (u *users) GetApprovedTutors() (users []*store.UserMgo, count int, err error) {
	query := u.Find(bson.M{
		"$and": []bson.M{
			{"role": store.RoleTutor},
			{"approval": store.ApprovalStatusApproved},
		},
	})

	if count, err = query.Count(); err != nil {
		return nil, count, err
	}

	if err = query.All(&users); err != nil {
		return nil, count, err
	}

	return
}

func (u *users) GetLessonsICS(page, limit int, q string, approvedOnly bool) (users []*store.UserMgo, count int, err error) {
	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = 50
	}
	searchRegex := bson.RegEx{
		Pattern: regexp.QuoteMeta(q),
		Options: "i",
	}

	approval := bson.M{
		"$or": []bson.M{
			{"approval": store.ApprovalStatusNew},
			{"approval": store.ApprovalStatusApproved},
			{"approval": store.ApprovalStatusBackgroundCheckRequested},
			{"approval": store.ApprovalStatusBackgroundCheckCompleted},
		},
	}
	if !approvedOnly {
		approval = bson.M{
			"approval": bson.M{"$exists": true},
		}
	}
	query := u.Find(bson.M{
		"$and": []bson.M{
			{"role": store.RoleTutor},
			approval,
		},
		"$or": []bson.M{
			{"username": searchRegex},
			{"profile.first_name": searchRegex},
			{"profile.last_name": searchRegex},
		},
	})
	if count, err = query.Count(); err != nil {
		return nil, count, err
	}
	if err = query.Sort("approval").Limit(limit).Skip((page - 1) * limit).All(&users); err != nil {
		return nil, count, err
	}
	return
}
