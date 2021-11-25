package store

import (
	"time"

	"github.com/golang-plus/errors"
	"gopkg.in/mgo.v2/bson"
)

type referBond byte

const (
	// NoBond represents a default value in case there's at least one different role than tutor and
	// student. Checking is strict, so administrators & root roles are affected.
	NoBond referBond = iota + 1
	// StudentToStudentBond represents the situation where a student invites a student. Should the
	// referral student complete a lesson, the referrer student gets a payout.
	StudentToStudentBond
	// StudentToTutorBond represents the situation where a student invites a tutor. Should the tutor
	// get approved and complete a lesson, the student gets a payout.
	StudentToTutorBond
	// TutorToStudentBond represents the situation where a tutor invites a student. Should the student
	// complete a lesson, the tutor gets a payout.
	TutorToStudentBond
	// TutorToTutorBond represents the situation where a tutor invites a tutor. Should the referral
	// tutor get approved and complete a lesson, the referrer tutor gets a payout.
	TutorToTutorBond
	// AffiliateToStudentBond represents the situation where an affiliate invites a student.
	AffiliateToStudentBond
	// AffiliateToTutorBond represents the situation where an affiliate invites a tutor.
	AffiliateToTutorBond
)

type referStep byte

const (
	// InvitedStep represents the default value which'll be assigned to invited users.
	// Does not satisfy any bond.
	InvitedStep referStep = iota + 1
	// SignedUpStep represents the value assigned to users who signed up using a referral code,
	// allowing them to receive the initial credit. Does not satisfy any bond.
	SignedUpStep
	// CompletedStep represents the value assigned to users who completed their contract.
	// That is, after students sign up and complete a lesson, or tutors get their account approved
	// and complete a lesson with total of 10 hours.
	CompletedStep
)

func (s referStep) String() string {
	switch s {
	case InvitedStep:
		return "hasn't signed up yet"
	case SignedUpStep:
		return "signed up"
	case CompletedStep:
		return "completed"
	default:
		return ""
	}
}

// Refer stores the referring data.
type Refer struct {
	// Referrer is the referral code of the user who invited the referral.
	Referrer string `json:"referrer,omitempty" bson:"referrer"`
	// ReferralCode is the user's invite code.
	ReferralCode string `json:"referral_code,omitempty" bson:"referral_code"`
}

// ReferLink represents the link between a referrer and its referral.
type ReferLink struct {
	ID          bson.ObjectId `json:"_id" bson:"_id"`
	CreatedAt   time.Time     `json:"created_at" bson:"created_at"`
	UpdatedAt   *time.Time    `json:"updated_at" bson:"updated_at"`
	CompletedAt *time.Time    `json:"completed_at" bson:"completed_at"`
	// Referrer is the referrer's user id. Used to link to the actual user model.
	Referrer *bson.ObjectId `json:"referrer" bson:"referrer"`
	// Referral is the referral's user id. Used to link to the actual user model.
	Referral *bson.ObjectId `json:"referral" bson:"referral"`
	// Affiliate represents whether the referrer is an affiliate user or not.
	Affiliate bool `json:"affiliate" bson:"affiliate"`
	// Email is set if we invite a user by sending him an email. Referral will be nil.
	Email string `json:"email" bson:"email"`
	// Bond is the contract between two users. Used to satisfy the refer's paying mechanism.
	Bond referBond `json:"bond" bson:"bond"`
	// Step is the latest step that the referral completed.
	Step referStep `json:"step" bson:"step"`
	// Amount is the credit that the referral will get upon registering.
	Amount float64 `json:"amount" bson:"amount"`
	// Satisfied represents whether the contract was satisfied or not.
	Satisfied bool `json:"satisfied" bson:"satisfied"`
	// Disabled represents whether the link was disabled in case a new one was created,
	// on re-inviting users, be it over the specified time span or force invite.
	Disabled bool `json:"disabled" bson:"disabled"`
}

// Insert adds a new entry to the refers collection.
func (r *ReferLink) Insert() (err error) {
	if !r.ID.Valid() {
		r.ID = bson.NewObjectId()
	}

	r.CreatedAt = time.Now()

	var referrerUser, referralUser *UserMgo
	err = GetCollection("users").FindId(r.Referrer).One(&referrerUser)
	if err != nil {
		return errors.Wrap(err, "can't find referrer by id")
	}

	// refer links added by email invites don't have a referral, but an email address
	// we also don't know the bond yet
	if r.Referral != nil {
		err = GetCollection("users").FindId(r.Referral).One(&referralUser)
		if err != nil {
			return errors.Wrap(err, "can't find referral by id")
		}

		switch true {
		case referrerUser.IsStudentStrict() && referralUser.IsStudentStrict():
			r.Bond = StudentToStudentBond
		case referrerUser.IsStudentStrict() && referralUser.IsTutorStrict():
			r.Bond = StudentToTutorBond
		case referrerUser.IsTutorStrict() && referralUser.IsTutorStrict():
			r.Bond = TutorToTutorBond
		case referrerUser.IsTutorStrict() && referralUser.IsStudentStrict():
			r.Bond = TutorToStudentBond
		case referrerUser.IsAffiliate() && referralUser.IsStudentStrict():
			r.Bond = AffiliateToStudentBond
		case referrerUser.IsAffiliate() && referralUser.IsTutorStrict():
			r.Bond = AffiliateToTutorBond
		default:
			r.Bond = NoBond
		}
	} else {
		r.Bond = NoBond
	}

	return GetCollection("refers").Insert(r)
}

// SetStep updates the refer link's step. Step must be one of the constants provided,
// and can't be smaller than the current step.
func (r *ReferLink) SetStep(s referStep) error {
	switch s {
	case InvitedStep, SignedUpStep, CompletedStep:
	default:
		return errors.New("invalid step provided")
	}

	if s < r.Step {
		return errors.New("can't downgrade step")
	}

	return GetCollection("refers").UpdateId(r.ID, bson.M{"$set": bson.M{
		"step": s, "updated_at": time.Now(),
	}})
}

// SetReferral updates the refer link's referral. Used when registering a user after getting an invite.
func (r *ReferLink) SetReferral(id *bson.ObjectId) error {
	r.Referral = id
	return GetCollection("refers").UpdateId(r.ID, bson.M{"$set": bson.M{
		"referral": id, "updated_at": time.Now(),
	}})
}

// SetAmount sets the amount of credit offered to the referral upon registering or satisfying the link.
func (r *ReferLink) SetAmount(a float64) error {
	r.Amount = a
	return GetCollection("refers").UpdateId(r.ID, bson.M{"$set": bson.M{
		"amount": a, "updated_at": time.Now(),
	}})
}

// SetAmount sets the amount of credit offered to the referral upon registering or satisfying the link.
func (r *ReferLink) SetBond(bond referBond) error {
	r.Bond = bond
	return GetCollection("refers").UpdateId(r.ID, bson.M{"$set": bson.M{
		"bond": bond, "updated_at": time.Now(),
	}})
}

// Complete sets the satisfied field of the struct to true.
func (r *ReferLink) Complete() error {
	now := time.Now()
	return GetCollection("refers").UpdateId(r.ID, bson.M{"$set": bson.M{
		"satisfied": true, "step": CompletedStep,
		"updated_at": now, "completed_at": now,
	}})
}
