package store

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"net/http"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/nyaruka/phonenumbers"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/utils/timeline"

	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/utils"

	jose "github.com/dvsekhvalnov/jose2go"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/mgo.v2/bson"
)

// Role uses bitmask to store what role(s) the user acts as in the system
type Role byte

// List of roles
const (
	RoleAffiliate Role = 1 << iota
	RoleStudent
	RoleTutor
	RoleAdmin
	RoleRoot
)

type ApprovalStatus byte

const (
	ApprovalStatusNew ApprovalStatus = iota + 1
	ApprovalStatusApproved
	ApprovalStatusRejected
	ApprovalStatusBackgroundCheckRequested
	ApprovalStatusBackgroundCheckCompleted
)

// Meet uses a bitmask to tell how a tutor can meet with students
type Meet byte

// List of ways to meet
const (
	MeetOnline Meet = 2 << iota
	MeetInPerson
	MeetBoth = MeetOnline | MeetInPerson
)

func (m Meet) String() string {
	switch m {
	case MeetOnline:
		return "meet online"
	case MeetInPerson:
		return "meet in-person"
	default:
		return "meet both"
	}
}

type UserPresence byte

// Used to tell if a tutor is currently online
const (
	Offline UserPresence = iota
	Online
	Iddle
)

type RegisteredEmail struct {
	Email    string     `json:"email" bson:"email"`
	Verified *time.Time `json:"verified,omitempty" bson:"verified,omitempty"`
	Created  time.Time  `json:"created,omitempty" bson:"created,omitempty"`
}

type PasswordAuthorization struct {
	Bcrypt string `json:"bcrypt" bson:"bcrypt"`
}

type AuthorizationServices struct {
	Password PasswordAuthorization `json:"password,omitempty" bson:"password,omitempty"`
	Secret   string                `json:"secret,omitempty" bson:"secret,omitempty"`
}

type Coordinates struct {
	Lng float64 `json:"lng" bson:"x"`
	Lat float64 `json:"lat" bson:"y"`
}

type GeoJSON struct {
	Type        string       `json:"type" bson:"type"`
	Coordinates *Coordinates `json:"coordinates" bson:"coordinates"`
}

type UserLocation struct {
	Position   *GeoJSON `json:"position,omitempty" bson:"position,omitempty"`
	Country    string   `json:"country,omitempty" bson:"country,omitempty"`
	State      string   `json:"state,omitempty" bson:"state,omitempty"`
	City       string   `json:"city,omitempty" bson:"city,omitempty"`
	Address    string   `json:"address,omitempty" bson:"address,omitempty"`
	PostalCode string   `json:"postal_code,omitempty" bson:"postal_code,omitempty"`
}

// Update sets new values to the UserLocation instance
func (loc *UserLocation) Update(newLoc *UserLocation) {
	if loc == nil {
		return
	}
	if newLoc.Position != nil {
		loc.Position = newLoc.Position
	}
	if newLoc.Country != "" {
		loc.Country = newLoc.Country
	}
	if newLoc.State != "" {
		loc.State = newLoc.State
	}
	if newLoc.City != "" {
		loc.City = newLoc.City
	}
	if newLoc.Address != "" {
		loc.Address = newLoc.Address
	}
	if newLoc.PostalCode != "" {
		loc.PostalCode = newLoc.PostalCode
	}
}

func (ul *UserLocation) String() string {
	var s bytes.Buffer

	if ul.Address != "" {
		s.WriteString(" ")
		s.WriteString(ul.Address)
	}

	if ul.City != "" {
		s.WriteString(", ")
		s.WriteString(ul.City)
	}

	if ul.State != "" {
		s.WriteString(", ")
		s.WriteString(ul.State)
	}

	if ul.PostalCode != "" {
		s.WriteString(" ")
		s.WriteString(ul.PostalCode)
	}

	if ul.Country != "" {
		s.WriteString(", ")
		s.WriteString(ul.Country)
	}

	return s.String()
}

type ReferralsDto struct {
	ID            bson.ObjectId  `json:"_id" bson:"_id"`
	User          *PublicUserDto `json:"user" bson:"user"`
	ReferralEmail string         `json:"referral_email" bson:"referral_email"`
	Step          referStep      `json:"step" bson:"step"`
	Reward        string         `json:"reward" bson:"reward"`
}

type UserReview struct {
	Communication   float64       `json:"communication" bson:"communication"`
	Clarity         float64       `json:"clarity" bson:"clarity"`
	Professionalism float64       `json:"professionalism" bson:"professionalism"`
	Patience        float64       `json:"patience" bson:"patience"`
	Helpfulness     float64       `json:"helpfulness" bson:"helpfulness"`
	ID              bson.ObjectId `json:"_id" bson:"_id"`
	User            bson.ObjectId `json:"user" bson:"user"`
	Reviewer        bson.ObjectId `json:"reviewer" bson:"reviewer"`
	Title           string        `json:"title" bson:"title"`
	PublicReview    string        `json:"public_review" bson:"public_review"`
	PrivateReview   string        `json:"private_review" bson:"private_review"`
	Approved        bool          `json:"approved" bson:"approved"`
	Time            time.Time     `json:"time" bson:"time"`
}

type UserReviewDto struct {
	Communication   float64       `json:"communication" bson:"communication"`
	Clarity         float64       `json:"clarity" bson:"clarity"`
	Professionalism float64       `json:"professionalism" bson:"professionalism"`
	Patience        float64       `json:"patience" bson:"patience"`
	Helpfulness     float64       `json:"helpfulness" bson:"helpfulness"`
	ID              bson.ObjectId `json:"_id" bson:"_id"`
	//User            *UserDto      `json:"user" bson:"user"`
	Reviewer      *PublicUserDto `json:"reviewer" bson:"reviewer"`
	Title         string         `json:"title" bson:"title"`
	PublicReview  string         `json:"public_review" bson:"public_review"`
	PrivateReview string         `json:"private_review" bson:"private_review"`
	Approved      bool           `json:"approved" bson:"approved"`
	Time          time.Time      `json:"time" bson:"time"`
}

type UserReviewRatings struct {
	Communication   float64 `json:"communication" bson:"communication"`
	Clarity         float64 `json:"clarity" bson:"clarity"`
	Professionalism float64 `json:"professionalism" bson:"professionalism"`
	Patience        float64 `json:"patience" bson:"patience"`
	Helpfulness     float64 `json:"helpfulness" bson:"helpfulness"`
}

type Profile struct {
	FirstName                    string     `json:"first_name" bson:"first_name"`
	LastName                     string     `json:"last_name" bson:"last_name"`
	About                        string     `json:"about,omitempty" bson:"about,omitempty"`
	Avatar                       *Upload    `json:"avatar,omitempty" bson:"avatar,omitempty"`
	Telephone                    string     `json:"telephone,omitempty" bson:"telephone,omitempty"`
	Resume                       string     `json:"resume,omitempty" bson:"resume,omitempty"`
	Birthday                     *time.Time `json:"birthday,omitempty" bson:"birthday,omitempty"`
	EmployerIdentificationNumber string     `json:"employer_identification_number,omitempty" bson:"employer_identification_number"`
	SocialSecurityNumber         string     `json:"social_security_number,omitempty" bson:"social_security_number"`
}

type TutoringDegree struct {
	ID          bson.ObjectId `json:"_id" bson:"_id"`
	Course      string        `json:"course" bson:"course" binding:"required"`
	Degree      string        `json:"degree" bson:"degree" binding:"required"`
	University  string        `json:"university" bson:"university" binding:"required"`
	Certificate *Upload       `json:"certificate" bson:"certificate"`
	Verified    bool          `json:"verified" bson:"verified"`
}

type TutoringDegreeDto struct {
	ID          bson.ObjectId `json:"_id" bson:"_id"`
	Course      string        `json:"course" bson:"course"`
	Degree      string        `json:"degree" bson:"degree"`
	University  string        `json:"university" bson:"university"`
	Certificate *Upload       `json:"certificate" bson:"certificate"`
	Verified    bool          `json:"verified" bson:"verified"`
}

type Favorite struct {
	Tutors   []FavoriteTutor   `json:"tutors" bson:"tutors"`
	Students []FavoriteStudent `json:"students" bson:"students"`
}

type FavoriteTutor struct {
	ID    bson.ObjectId `json:"_id" bson:"_id"`
	Tutor bson.ObjectId `json:"tutor" bson:"tutor"`
}

type FavoriteStudent struct {
	ID      bson.ObjectId `json:"_id" bson:"_id"`
	Student bson.ObjectId `json:"student" bson:"student"`
}

type University struct {
	ID          bson.ObjectId `json:"_id" bson:"_id"`
	Name        string        `json:"name" bson:"name"`
	CountryCode string        `json:"country_code" bson:"country_code"`
	Country     string        `json:"country" bson:"country"`
	Website     string        `json:"website,omitempty" bson:"website,omitempty"`
}

type Subject struct {
	ID   bson.ObjectId `json:"_id" bson:"_id,omitempty"`
	Name string        `json:"name" bson:"subject"`
}

type availabilityState int

const (
	Available   availabilityState = 1
	Unavailable availabilityState = 0
)

type GeneralAvailability int32

const (
	MondayMorning GeneralAvailability = 1 << (iota + 1)
	MondayAfternoon
	MondayEvening

	TuesdayMorning
	TuesdayAfternoon
	TuesdayEvening

	WednesdayMorning
	WednesdayAfternoon
	WednesdayEvening

	ThursdayMorning
	ThursdayAfternoon
	ThursdayEvening

	FridayMorning
	FridayAfternoon
	FridayEvening

	SaturdayMorning
	SaturdayAfternoon
	SaturdayEvening

	SundayMorning
	SundayAfternoon
	SundayEvening
)

const (
	MondayAllDay     = MondayMorning | MondayAfternoon | MondayEvening
	MondayFirstHalf  = MondayMorning | MondayAfternoon
	MondaySecondHalf = MondayAfternoon | MondayEvening
	MondayEnds       = MondayMorning | MondayEvening

	TuesdayAllDay     = TuesdayMorning | TuesdayAfternoon | TuesdayEvening
	TuesdayFirstHalf  = TuesdayMorning | TuesdayAfternoon
	TuesdaySecondHalf = TuesdayAfternoon | TuesdayEvening
	TuesdayEnds       = TuesdayMorning | TuesdayEvening

	WednesdayAllDay     = WednesdayMorning | WednesdayAfternoon | WednesdayEvening
	WednesdayFirstHalf  = WednesdayMorning | WednesdayAfternoon
	WednesdaySecondHalf = WednesdayAfternoon | WednesdayEvening
	WednesdayEnds       = WednesdayMorning | WednesdayEvening

	ThursdayAllDay     = ThursdayMorning | ThursdayAfternoon | ThursdayEvening
	ThursdayFirstHalf  = ThursdayMorning | ThursdayAfternoon
	ThursdaySecondHalf = ThursdayAfternoon | ThursdayEvening
	ThursdayEnds       = ThursdayMorning | ThursdayEvening

	FridayAllDay     = FridayMorning | FridayAfternoon | FridayEvening
	FridayFirstHalf  = FridayMorning | FridayAfternoon
	FridaySecondHalf = FridayAfternoon | FridayEvening
	FridayEnds       = FridayMorning | FridayEvening

	SaturdayAllDay     = SaturdayMorning | SaturdayAfternoon | SaturdayEvening
	SaturdayFirstHalf  = SaturdayMorning | SaturdayAfternoon
	SaturdaySecondHalf = SaturdayAfternoon | SaturdayEvening
	SaturdayEnds       = SaturdayMorning | SaturdayEvening

	SundayAllDay     = SundayMorning | SundayAfternoon | SundayEvening
	SundayFirstHalf  = SundayMorning | SundayAfternoon
	SundaySecondHalf = SundayAfternoon | SundayEvening
	SundayEnds       = SundayMorning | SundayEvening
)

type TutoringSubject struct {
	ID          bson.ObjectId `json:"_id" bson:"_id"`
	Subject     bson.ObjectId `json:"subject" bson:"subject" binding:"required"`
	Certificate *Upload       `json:"certificate,omitempty" bson:"certificate"`
	Verified    bool          `json:"verified" bson:"verified"`
}

type TutoringSubjectDto struct {
	ID          bson.ObjectId `json:"_id" bson:"_id"`
	Subject     Subject       `json:"subject" bson:"subject"`
	Certificate *Upload       `json:"certificate,omitempty" bson:"certificate"`
	Verified    bool          `json:"verified" bson:"verified"`
}

type Tutoring struct {
	Rate                float32           `json:"rate" bson:"rate,omitempty"`
	LessonBuffer        int               `json:"lesson_buffer" bson:"lesson_buffer"`
	Rating              float32           `json:"rating" bson:"rating"`
	Reviewers           float32           `json:"reviewers" bson:"reviewers"`
	InstantSession      bool              `json:"instant_session" bson:"instant_session"`
	InstantBooking      bool              `json:"instant_booking" bson:"instant_booking"`
	Meet                Meet              `json:"meet" bson:"meet,omitempty"`
	Availability        *Availability     `json:"availability" bson:"availability,omitempty"`
	Blackout            *Availability     `json:"blackout" bson:"blackout,omitempty"`
	Degrees             []TutoringDegree  `json:"degrees" bson:"degrees"`
	Subjects            []TutoringSubject `json:"subjects" bson:"subjects"`
	Title               string            `json:"title,omitempty" bson:"title,omitempty"`
	Video               *Upload           `json:"video,omitempty" bson:"video"`
	Resume              *Upload           `json:"resume" bson:"resume"`
	YouTubeVideo        string            `json:"youtube_video,omitempty" bson:"youtube_video,omitempty"`
	PromoteVideoAllowed bool              `json:"promote_video_allowed,omitempty" bson:"promote_video_allowed,omitempty"`
	ProfileChecked      *time.Time        `json:"profile_checked,omitempty" bson:"profile_checked,omitempty"`
}

type TutoringDto struct {
	Rate                float32              `json:"rate" bson:"rate,omitempty"`
	Buffer              int                  `json:"lesson_buffer" bson:"lesson_buffer"`
	Rating              float32              `json:"rating" bson:"rating"`
	Reviewers           float32              `json:"reviewers" bson:"reviewers"`
	InstantSession      bool                 `json:"instant_session" bson:"instant_session"`
	InstantBooking      bool                 `json:"instant_booking" bson:"instant_booking"`
	Meet                Meet                 `json:"meet" bson:"meet,omitempty"`
	Availability        *Availability        `json:"availability,omitempty" bson:"availability,omitempty"`
	Degrees             []TutoringDegreeDto  `json:"degrees" bson:"degrees"`
	Subjects            []TutoringSubjectDto `json:"subjects" bson:"subjects"`
	Title               string               `json:"title,omitempty" bson:"title,omitempty"`
	Video               *Upload              `json:"video,omitempty" bson:"video"`
	YouTubeVideo        string               `json:"youtube_video,omitempty" bson:"youtube_video,omitempty"`
	Resume              *Upload              `json:"resume,omitempty" bson:"resume"`
	HoursTaught         float64              `json:"hours_taught"`
	PromoteVideoAllowed bool                 `json:"promote_video_allowed,omitempty" bson:"promote_video_allowed,omitempty"`
	ProfileChecked      *time.Time           `json:"profile_checked,omitempty" bson:"profile_checked,omitempty"`
}

// UserCard represents the structure of a credit card that gets inserted in the database.
// Includes data from Stripe when adding it.
type UserCard struct {
	ID      string `json:"id" bson:"id"`
	Number  string `json:"number" bson:"number"`
	Year    uint16 `json:"year" bson:"year"`
	Month   uint8  `json:"month" bson:"month"`
	Type    string `json:"type" bson:"type"`
	Default bool   `json:"default" bson:"default"`
}

type BankAccount struct {
	BankAccountID      string `json:"bank_account_id" bson:"bank_account_id"`
	BankAccountName    string `json:"bank_account_name" bson:"bank_account_name"`
	BankAccountType    string `json:"bank_account_type" bson:"bank_account_type"`
	BankAccountNumber  string `json:"bank_account_number" bson:"bank_account_number"`
	BankAccountRouting string `json:"bank_account_routing" bson:"bank_account_routing"`
}

type Payments struct {
	CustomerID string      `json:"customer" bson:"customer"`
	ConnectID  string      `json:"connect" bson:"connect"`
	Cards      []*UserCard `json:"cards" bson:"cards"`
	Credits    int64       `json:"credits,omitempty" bson:"credits,omitempty"`
	BankAccount
}

type SurveyDetails struct {
	OpenedAt    time.Time  `json:"opened_at" bson:"opened_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
}

// Update will update the Profile instance with new values
func (p *Profile) Update(newProfile *Profile) (err error) {
	if p == nil {
		return
	}
	if newProfile.Avatar != nil {
		p.Avatar = newProfile.Avatar
	}
	if newProfile.FirstName != "" {
		p.FirstName = newProfile.FirstName
	}
	if newProfile.LastName != "" {
		p.LastName = newProfile.LastName
	}
	if newProfile.Birthday != nil {
		p.Birthday = newProfile.Birthday
	}
	if newProfile.Telephone != "" {
		phone, err := phonenumbers.Parse(newProfile.Telephone, "US")
		if err != nil {
			return err
		}
		if !phonenumbers.IsValidNumber(phone) {
			return errors.New("invalid phone number")
		}
		p.Telephone = newProfile.Telephone
	}
	return nil
}

func getUser(id bson.ObjectId) (u *UserMgo, err error) {
	err = GetCollection("users").FindId(id).One(&u)
	return
}

// FindAll users that match the IDs
func getUsers(IDs []bson.ObjectId) ([]*UserMgo, error) {

	var users []*UserMgo

	query := GetCollection("users").Find(bson.M{
		"_id": bson.M{"$in": IDs},
	})

	if err := query.All(&users); err != nil {
		return nil, errors.New("could not get all users")
	}

	return users, nil
}

func getUserDto(id bson.ObjectId) (*PublicUserDto, error) {
	u, err := getUser(id)
	if err != nil {
		return nil, err
	}
	return u.ToPublicDto(), nil
}

// Dto converts from the mongo version by filling in full user data
func (ur *UserReview) Dto() *UserReviewDto {
	dto := &UserReviewDto{
		Communication:   ur.Communication,
		Clarity:         ur.Clarity,
		Professionalism: ur.Professionalism,
		Patience:        ur.Patience,
		Helpfulness:     ur.Helpfulness,
		ID:              ur.ID,
		//User:            &UserDto{ID: ur.User},
		Reviewer:      &PublicUserDto{ID: ur.Reviewer},
		Title:         ur.Title,
		PublicReview:  ur.PublicReview,
		PrivateReview: ur.PrivateReview,
		Approved:      ur.Approved,
		Time:          ur.Time,
	}

	var err error
	//dto.User, err = getUserDto(ur.User)
	//if err != nil {
	//	logger.Get().Errorf("[!] Could not lookup review user", err)
	//}

	err = nil
	dto.Reviewer, err = getUserDto(ur.Reviewer)
	if err != nil {
		logger.Get().Errorf("Could not lookup review user: %v", err)
	}
	return dto
}

func (u *UserMgo) TutoringDto() (tutoring *TutoringDto) {
	if u.Tutoring == nil {
		return nil
	}

	degrees := make([]TutoringDegreeDto, 0)
	subjects := make([]TutoringSubjectDto, 0)

	var ids []bson.ObjectId

	for _, sub := range u.Tutoring.Subjects {
		ids = append(ids, sub.Subject)
	}

	for _, degree := range u.Tutoring.Degrees {
		degrees = append(degrees, TutoringDegreeDto{
			ID:          degree.ID,
			Course:      degree.Course,
			Degree:      degree.Degree,
			University:  degree.University,
			Certificate: degree.Certificate,
			Verified:    degree.Verified,
		})
	}

	for _, sub := range u.Tutoring.Subjects {
		var subject Subject
		GetCollection("subjects").FindId(sub.Subject).One(&subject)

		subjects = append(subjects, TutoringSubjectDto{
			ID:          sub.ID,
			Subject:     subject,
			Certificate: sub.Certificate,
			Verified:    sub.Verified,
		})
	}

	var hoursTaught float64
	for _, l := range u.GetLessons(true) {
		if l.State != LessonCompleted {
			continue
		}
		hoursTaught += l.Duration().Hours()
	}

	if sessions, err := u.GetInstantSessions(true); err == nil {
		for _, s := range sessions {
			if s.State != LessonCompleted {
				continue
			}
			hoursTaught += s.Duration().Hours()
		}
	}

	return &TutoringDto{
		Availability:        u.Tutoring.Availability,
		Rate:                u.Tutoring.Rate,
		Buffer:              u.Tutoring.LessonBuffer,
		Meet:                u.Tutoring.Meet,
		Rating:              u.Tutoring.Rating,
		Reviewers:           u.Tutoring.Reviewers,
		Degrees:             degrees,
		Subjects:            subjects,
		Video:               u.Tutoring.Video,
		YouTubeVideo:        u.Tutoring.YouTubeVideo,
		Resume:              u.Tutoring.Resume,
		InstantSession:      u.Tutoring.InstantSession,
		InstantBooking:      u.Tutoring.InstantBooking,
		HoursTaught:         hoursTaught,
		Title:               u.Tutoring.Title,
		PromoteVideoAllowed: u.Tutoring.PromoteVideoAllowed,
	}
}

type UserPreferences struct {
	/* User accepted terms from account payout*/
	PayoutTermsAccepted bool `json:"payout_terms_accepted" bson:"payout_terms_accepted"`

	/* Receive email updates */
	ReceiveUpdates bool `json:"receive_updates" bson:"receive_updates"`
	/* Receive sms updates */
	ReceiveSMSUpdates bool `json:"receive_sms_updates" bson:"receive_sms_updates"`

	/* Is Publicly Searchable? */
	IsPrivate bool `json:"is_private" bson:"is_private"`
}

func (u *UserMgo) UpdatePreferences(p *UserPreferences) (err error) {
	if p == nil {
		return fmt.Errorf("user preferences can't be nil")
	}

	u.Preferences = p

	err = GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{"preferences": u.Preferences}})
	if err != nil {
		return fmt.Errorf("couldn't update preferences: %s", err)
	}

	return nil
}

func (u *UserMgo) IsReceiveUpdates() bool {
	return u.Preferences.ReceiveUpdates
}

func (u *UserMgo) IsReceiveSMSUpdates() bool {
	return u.Preferences.ReceiveSMSUpdates
}

type LoginDetails struct {
	IP     string    `json:"ip" bson:"ip"`
	Device string    `json:"device" bson:"device"`
	Time   time.Time `json:"time" bson:"time"`
}

type UserCheckrData struct {
	CandidateID string `json:"candidate_id" bson:"candidate_id"`
	ReportID    string `json:"report_id" bson:"report_id,omitempty"`
	Finished    bool   `json:"finished" bson:"finished,omitempty"`
	Status      string `json:"status" bson:"status,omitempty"`
}

type UserBGCheckData struct {
	// CandidateID is the full UUID like d54eb469-d74d-450a-b7f9-b0d98f55ac9e
	CandidateID string `json:"candidate_id" bson:"candidate_id"`
	// ShortID is the shortened candidate ID like C1704355400
	ShortID  string `json:"short_id" bson:"short_id,omitempty"`
	Finished bool   `json:"complete" bson:"finished,omitempty"`
	State    string `json:"state" bson:"state,omitempty"`
}

type NoteType string

const (
	RejectType = NoteType("rejected")
	MiscType   = NoteType("miscellaneous")
)

type NoteUpdatedBy struct {
	ID   bson.ObjectId `json:"_id" bson:"_id"`
	Name string        `json:"name" bson:"name"`
}

type UserNote struct {
	ID        bson.ObjectId `json:"_id" bson:"_id"`
	Type      *NoteType     `json:"note_type,omitempty" bson:"type,omitempty"`
	Note      string        `json:"note" bson:"note"`
	CreatedAt time.Time     `json:"created_at" bson:"created_at"`
	UpdatedBy NoteUpdatedBy `json:"updated_by" bson:"updated_by"`
}

type Intercom struct {
	ContactId   string `json:"contact,omitempty" bson:"contact,omitempty"`
	WorkspaceId string `json:"workspace,omitempty" bson:"workspace,omitempty"`
}

type AccessToken struct {
	Token        string    `json:"token,omitempty" bson:"token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty" bson:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty" bson:"expiry,omitempty"`
}

type SocialNetwork struct {
	Network         string       `json:"network,omitempty" bson:"network,omitempty"`
	Sub             string       `json:"sub,omitempty" bson:"sub,omitempty"`
	LastAccessToken *AccessToken `json:"last_access_token,omitempty" bson:"last_access_token,omitempty"`
}

type UserMgo struct {
	ID                bson.ObjectId            `json:"_id" bson:"_id"`
	Username          string                   `json:"username,omitempty" bson:"username"`
	Services          AuthorizationServices    `json:"-" bson:"services"`
	Profile           Profile                  `json:"profile,omitempty" bson:"profile"`
	Tutoring          *Tutoring                `json:"tutoring,omitempty" bson:"tutoring,omitempty"`
	Emails            []RegisteredEmail        `json:"emails,omitempty" bson:"emails"`
	Role              Role                     `json:"role,omitempty" bson:"role"`
	Payments          *Payments                `json:"payments,omitempty" bson:"payments,omitempty"`
	Location          *UserLocation            `json:"location,omitempty" bson:"location,omitempty"`
	Timezone          string                   `json:"timezone,omitempty" bson:"timezone,omitempty"`
	Online            UserPresence             `json:"online,omitempty" bson:"online"`
	Preferences       *UserPreferences         `json:"preferences,omitempty" bson:"preferences,omitempty"`
	RegisteredDate    *time.Time               `json:"registered_date,omitempty" bson:"registered_date"`
	LastLogin         *LoginDetails            `json:"last_login,omitempty" bson:"last_login"`
	Disabled          bool                     `json:"disabled,omitempty" bson:"disabled"`
	Refer             *Refer                   `json:"refer,omitempty" bson:"refer"`
	CheckrData        *UserCheckrData          `json:"-" bson:"checkr_data"`
	BGCheckData       *UserBGCheckData         `json:"-" bson:"bg_check_data"`
	Surveys           map[string]SurveyDetails `json:"surveys,omitempty" bson:"surveys,omitempty"`
	Notes             []UserNote               `json:"notes,omitempty" bson:"notes,omitempty"`
	ApprovalStatus    ApprovalStatus           `json:"approval,omitempty" bson:"approval,omitempty"`
	ApprovalUpdatedAt *time.Time               `json:"approval_updated_at,omitempty" bson:"approval_updated_at,omitempty"`
	Intercom          *Intercom                `json:"intercom,omitempty" bson:"intercom,omitempty"`
	IsTestAccount     bool                     `json:"is_test_account,omitempty" bson:"is_test_account,omitempty"`
	Favorite          Favorite                 `json:"favorite,omitempty" bson:"favorite,omitempty"`
	SocialNetworks    []SocialNetwork          `json:"social_networks,omitempty" bson:"social_networks,omitempty"`
	Files             []bson.ObjectId          `json:"files,omitempty" bson:"files,omitempty"`
}

type UserDto struct {
	ID                bson.ObjectId            `json:"_id" bson:"_id"`
	Username          string                   `json:"username,omitempty" bson:"username"`
	Profile           Profile                  `json:"profile" bson:"profile"`
	Emails            []RegisteredEmail        `json:"emails,omitempty" bson:"emails"`
	Location          *UserLocation            `json:"location,omitempty" bson:"location"`
	Tutoring          *TutoringDto             `json:"tutoring,omitempty" bson:"tutoring"`
	Timezone          string                   `json:"timezone,omitempty" bson:"timezone"`
	Online            UserPresence             `json:"online" bson:"online"`
	Preferences       *UserPreferences         `json:"preferences,omitempty" bson:"preferences"`
	Role              Role                     `json:"role,omitempty" bson:"role,omitempty"`
	Payments          *Payments                `json:"payments,omitempty" bson:"payments"`
	RegisteredDate    *time.Time               `json:"registered_date,omitempty" bson:"registered_date,omitempty"`
	LastLogin         *LoginDetails            `json:"last_login,omitempty" bson:"last_login"`
	CC                bool                     `json:"cc,omitempty" bson:"cc,omitempty"`
	Refer             *Refer                   `json:"refer,omitempty" bson:"refer"`
	Disabled          bool                     `json:"disabled" bson:"disabled"`
	HasCheckrData     *bool                    `json:"has_checkr_data,omitempty"`
	HasBGCheckData    *bool                    `json:"has_bgcheck_data,omitempty"`
	Surveys           map[string]SurveyDetails `json:"surveys,omitempty" bson:"surveys,omitempty"`
	Notes             []UserNote               `json:"notes,omitempty" bson:"notes,omitempty"`
	ApprovalStatus    ApprovalStatus           `json:"approval,omitempty" bson:"approval,omitempty"`
	ApprovalUpdatedAt *time.Time               `json:"approval_updated_at,omitempty" bson:"approval_updated_at,omitempty"`
	Intercom          *Intercom                `json:"intercom,omitempty" bson:"intercom,omitempty"`
	IsTestAccount     bool                     `json:"is_test_account,omitempty" bson:"is_test_account,omitempty"`
	Favorite          Favorite                 `json:"favorite,omitempty" bson:"favorite,omitempty"`
	SocialNetworks    []SocialNetwork          `json:"social_networks,omitempty" bson:"social_networks,omitempty"`
	Files             []bson.ObjectId          `json:"files,omitempty" bson:"files,omitempty"`
}

type PublicTutoringDto struct {
	Rate           float32              `json:"rate" bson:"rate,omitempty"`
	Buffer         int                  `json:"lesson_buffer" bson:"lesson_buffer"`
	Rating         float32              `json:"rating" bson:"rating"`
	Reviewers      float32              `json:"reviewers" bson:"reviewers"`
	InstantSession bool                 `json:"instant_session" bson:"instant_session"`
	InstantBooking bool                 `json:"instant_booking" bson:"instant_booking"`
	Meet           Meet                 `json:"meet" bson:"meet,omitempty"`
	Degrees        []TutoringDegreeDto  `json:"degrees" bson:"degrees"`
	Subjects       []TutoringSubjectDto `json:"subjects" bson:"subjects"`
	Title          string               `json:"title,omitempty" bson:"title,omitempty"`
	HoursTaught    float64              `json:"hours_taught"`
}

type PublicUserLocation struct {
	Country string `json:"country,omitempty" bson:"country,omitempty"`
	State   string `json:"state,omitempty" bson:"state,omitempty"`
	City    string `json:"city,omitempty" bson:"city,omitempty"`
}

type PublicUserProfile struct {
	FirstName string  `json:"first_name" bson:"first_name"`
	LastName  string  `json:"last_name" bson:"last_name"`
	About     string  `json:"about,omitempty" bson:"about,omitempty"`
	Avatar    *Upload `json:"avatar,omitempty" bson:"avatar,omitempty"`
}

type PublicUserEmail struct {
	Email string `json:"email" bson:"email"`
}

type PublicUserDto struct {
	ID       bson.ObjectId       `json:"_id" bson:"_id"`
	Profile  *PublicUserProfile  `json:"profile" bson:"profile"`
	Emails   []PublicUserEmail   `json:"emails,omitempty" bson:"emails"`
	Location *PublicUserLocation `json:"location,omitempty" bson:"location"`
	Tutoring *PublicTutoringDto  `json:"tutoring,omitempty" bson:"tutoring"`
	Timezone string              `json:"timezone,omitempty" bson:"timezone"`
	Online   UserPresence        `json:"online" bson:"online"`
	Role     Role                `json:"role,omitempty" bson:"role,omitempty"`
	Refer    *Refer              `json:"refer,omitempty" bson:"refer"`
}

func (u *UserMgo) ToPublicDto() *PublicUserDto {
	profile := u.Profile
	location := u.Location
	tutoring := u.Tutoring
	emails := u.Emails
	dto := &PublicUserDto{
		ID: u.ID,
		Profile: &PublicUserProfile{
			FirstName: profile.FirstName,
			LastName:  profile.LastName,
			About:     profile.About,
			Avatar:    profile.Avatar,
		},
		Emails:   make([]PublicUserEmail, 0),
		Location: &PublicUserLocation{},
		Timezone: u.Timezone,
		Online:   u.Online,
		Role:     u.Role,
		Refer:    u.Refer,
	}
	if location != nil {
		dto.Location.Country = location.Country
		dto.Location.State = location.State
		dto.Location.City = location.City
	}
	if emails != nil {
		for _, e := range emails {
			dto.Emails = append(dto.Emails, PublicUserEmail{Email: e.Email})
		}
	}
	if tutoring != nil {
		degrees := make([]TutoringDegreeDto, 0)
		subjects := make([]TutoringSubjectDto, 0)
		for _, degree := range tutoring.Degrees {
			degrees = append(degrees, TutoringDegreeDto{
				ID:          degree.ID,
				Course:      degree.Course,
				Degree:      degree.Degree,
				University:  degree.University,
				Certificate: degree.Certificate,
				Verified:    degree.Verified,
			})
		}
		for _, subject := range tutoring.Subjects {
			var sub Subject
			GetCollection("subjects").FindId(subject.Subject).One(&sub)
			subjects = append(subjects, TutoringSubjectDto{
				ID:          subject.ID,
				Subject:     sub,
				Certificate: subject.Certificate,
				Verified:    subject.Verified,
			})
		}

		var hoursTaught float64
		for _, l := range u.GetLessons(true) {
			if l.State != LessonCompleted {
				continue
			}
			hoursTaught += l.Duration().Hours()
		}

		if sessions, err := u.GetInstantSessions(true); err == nil {
			for _, s := range sessions {
				if s.State != LessonCompleted {
					continue
				}
				hoursTaught += s.Duration().Hours()
			}
		}

		dto.Tutoring = &PublicTutoringDto{
			Rate:           tutoring.Rate,
			Buffer:         tutoring.LessonBuffer,
			Rating:         tutoring.Rating,
			Reviewers:      tutoring.Reviewers,
			InstantSession: tutoring.InstantSession,
			InstantBooking: tutoring.InstantBooking,
			Meet:           tutoring.Meet,
			Degrees:        degrees,
			Subjects:       subjects,
			Title:          tutoring.Title,
			HoursTaught:    hoursTaught,
		}
	}
	return dto
}

func (u *UserMgo) ToThreadParticipant() *ThreadParticipant {
	return &ThreadParticipant{
		ID: u.ID,
		Profile: ThreadParticipantProfile{
			FirstName: u.Profile.FirstName,
			LastName:  u.Profile.LastName,
			Avatar:    u.Profile.Avatar,
		},
	}
}

func (u *UserMgo) SaveSurveyLastOpenedAtTime(openedTime time.Time) error {
	return GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{"survey_last_opened_at": openedTime}})
}

func (u *UserMgo) CreateNote(note UserNote) error {
	return GetCollection("users").UpdateId(u.ID, bson.M{"$push": bson.M{"notes": note}})
}

func (u *UserMgo) UpdateApprovalStatus(status ApprovalStatus) error {
	return GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{
		"approval_updated_at": time.Now().UTC(),
		"approval":            status,
	}})
}

// Dto fills in the IDs from what is returned by the database
func (u *UserMgo) Dto(sensitive ...bool) *UserDto {
	dto := &UserDto{
		ID: u.ID,
		Profile: Profile{
			FirstName: u.Profile.FirstName,
			LastName:  u.Profile.LastName,
			About:     u.Profile.About,
			Avatar:    u.Profile.Avatar,
		},
		Location:      u.Location,
		Tutoring:      u.TutoringDto(),
		Timezone:      u.Timezone,
		Online:        u.Online,
		Role:          u.Role,
		Disabled:      u.Disabled,
		IsTestAccount: u.IsTestAccount,
		Favorite:      u.Favorite,
	}

	if len(sensitive) > 0 && sensitive[0] {
		dto.Username = u.Username
		dto.Profile = u.Profile
		dto.Emails = u.Emails
		dto.Preferences = u.Preferences
		dto.LastLogin = u.LastLogin
		dto.CC = u.HasCreditCard()
		dto.Refer = u.Refer
		dto.Payments = u.Payments
		dto.RegisteredDate = u.RegisteredDate
		dto.Intercom = u.Intercom
		if u.CheckrData != nil {
			t := true
			dto.HasCheckrData = &t
		}
		if u.BGCheckData != nil {
			t := true
			dto.HasBGCheckData = &t
		}
		dto.Location = u.Location
		dto.Surveys = u.Surveys
		dto.Notes = u.Notes
		if u.IsTutor() {
			dto.ApprovalStatus = u.ApprovalStatus
			dto.ApprovalUpdatedAt = u.ApprovalUpdatedAt
		}
	} else {
		if dto.Location != nil {
			dto.Location.PostalCode = ""
			dto.Location.Position = nil
			dto.Location.Address = ""
		}

		if dto.Tutoring != nil {
			dto.Tutoring.Resume = nil
		}
	}

	return dto
}

// SaveNew sets reasonable defaults ID, LessonBuffer, and SSN, standardizes the name capitalization, and inserts the user
func (u *UserMgo) SaveNew() (err error) {
	if !u.ID.Valid() {
		u.ID = bson.NewObjectId()
	}

	if u.Tutoring != nil && u.Tutoring.LessonBuffer == 0 {
		u.Tutoring.LessonBuffer = 15
	}

	if u.Profile.SocialSecurityNumber != "" {
		// Mask out the SSN before saving
		ssn := u.Profile.SocialSecurityNumber
		u.Profile.SocialSecurityNumber = fmt.Sprintf("%s%s", "###-##-", ssn[len(ssn)-4:])
	}

	u.Profile.FirstName = strings.Title(u.Profile.FirstName)
	u.Profile.LastName = strings.Title(u.Profile.LastName)
	return GetCollection("users").Insert(u)
}

func (u *UserMgo) DisableAccount() (err error) {
	u.Disabled = true
	return GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{"disabled": true}})
}

func (u *UserMgo) SetLastAccessToken(network, sub, token string) (err error) {
	return GetCollection("users").Update(
		bson.M{
			"_id": u.ID,
			"$and": []bson.M{
				{"social_networks.social.network": network},
				{"social_networks.social.sub": sub},
			},
		},
		bson.M{
			"$set": bson.M{
				"social_networks.social.$.last_access_token": token,
			},
		},
	)
}

func (u *UserMgo) SetLoginDetails(c *gin.Context) {
	details := &LoginDetails{}
	details.IP = utils.GetIP(c)
	details.Device = c.Request.Header.Get("User-Agent")
	details.Time = time.Now()

	GetCollection("users").UpdateId(u.ID, bson.M{
		"$set": bson.M{
			"last_login": details,
		},
	})

	if u.Refer.ReferralCode == "" {
		if err := u.SetReferralCode(true); err != nil {
			c.JSON(http.StatusBadRequest, core.NewErrorResponse(utils.CapitalizeFirstWord(err.Error())))
		}
	}
}

// HasBank returns whether a user has the payout details set.
func (u *UserMgo) HasBank() bool {
	if u.Payments == nil || u.Payments.ConnectID == "" {
		return false
	}

	if u.Payments.BankAccountNumber == "" {
		return false
	}

	if u.Payments.BankAccountRouting == "" {
		return false
	}

	return true
}

// HasCreditCard returns whether a user has a credit card attached to the account.
func (u *UserMgo) HasCreditCard() bool {
	if u.Payments == nil || u.Payments.CustomerID == "" {
		return false
	}

	return len(u.Payments.Cards) > 0
}

// Avatar returns the user's avatar address or the default.
func (u *UserMgo) Avatar() string {
	if u.Profile.Avatar != nil {
		return u.Profile.Avatar.Href()
	}

	return "https://s3.amazonaws.com/tutorthepeople/temp/default-avatar.png"
}

// SetPaymentsCustomer sets sets up a customer with an account
func (u *UserMgo) SetPaymentsCustomer(customerID string) (err error) {
	ref := &Payments{}

	if u.Payments == nil {
		u.Payments = ref
	}

	u.Payments.CustomerID = customerID

	return GetCollection("users").UpdateId(u.ID, bson.M{
		"$set": bson.M{
			"payments.customer": customerID,
		},
	})
}

// SetPaymentsConnect sets sets up user with connect account
func (u *UserMgo) SetPaymentsConnect(connectID string) (err error) {
	ref := &Payments{}

	if u.Payments == nil {
		u.Payments = ref
	}

	u.Payments.ConnectID = connectID

	return GetCollection("users").UpdateId(u.ID, bson.M{
		"$set": bson.M{
			"payments.connect": connectID,
		},
	})
}

// SetCards will update the database with card info
func (u *UserMgo) SetCards(cards []*UserCard) error {
	if u.Payments == nil {
		return errors.New("cannot set cards since there is no payment account set on the user")
	}
	u.Payments.Cards = cards

	return GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{"payments.cards": cards}})
}

// SetBankAccount will set the database with the reduced bankaccount
func (u *UserMgo) SetBankAccount(ba BankAccount) error {
	if u.Payments == nil {
		return errors.New("cannot set bank account since there is no payment account set on the user")
	}
	u.Payments.BankAccount = ba

	return GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{"payments.bankaccount": ba}})
}

// SetBankAccount will set the database with the deleted bankaccount
func (u *UserMgo) DeleteBankAccount() error {
	if u.Payments == nil {
		return errors.New("cannot set bank account since there is no payment account set on the user")
	}
	u.Payments.BankAccount = BankAccount{}

	return GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{"payments.bankaccount": BankAccount{}}})
}

func (u *UserMgo) CountReviews() int {
	n, _ := GetCollection("reviews").Find(bson.M{"user": u.ID}).Count()
	return n
}

func (u *UserMgo) AverageReviews() (reviews *UserReviewRatings) {
	reviews = &UserReviewRatings{}

	all := make([]UserReviewRatings, 0)

	if err := GetCollection("reviews").Find(bson.M{"user": u.ID}).All(&all); err != nil {
		logger.Get().Errorf("failed to find users: %v", err)
		return
	}

	if len(all) == 0 {
		return nil
	}

	for _, item := range all {
		reviews.Communication += item.Communication
		reviews.Clarity += item.Clarity
		reviews.Helpfulness += item.Helpfulness
		reviews.Patience += item.Patience
		reviews.Professionalism += item.Professionalism
	}

	size := float64(len(all))
	reviews.Communication /= size
	reviews.Clarity /= size
	reviews.Helpfulness /= size
	reviews.Patience /= size
	reviews.Professionalism /= size

	return reviews
}

func (u *UserMgo) Reviews(limit, offset int) (reviewsDto []*UserReviewDto) {
	q := GetCollection("reviews").Pipe([]bson.M{
		{"$match": bson.M{"user": u.ID}},
		{
			"$project": bson.M{
				"user":            1,
				"reviewer":        1,
				"clarity":         1,
				"communication":   1,
				"helpfulness":     1,
				"patience":        1,
				"professionalism": 1,
				"title":           1,
				"public_review":   1,
				"time":            1,
			},
		},
		{"$sort": bson.M{"time": -1}},
		{"$skip": offset},
		{"$limit": limit},
	})

	var reviews = make([]UserReview, 0)

	if err := q.All(&reviews); err != nil {
		logger.Get().Errorf("Error getting reviews: %v", err)
		return
	}

	reviewsDto = make([]*UserReviewDto, len(reviews))

	for i, review := range reviews {
		reviewsDto[i] = review.Dto()
	}

	return
}

func (u *UserMgo) AddReview(review *UserReview) (err error) {
	return GetCollection("reviews").Insert(review)
}

// Name returns the first and the last name.
func (u *UserMgo) Name() string {
	return strings.Title(u.Profile.FirstName + " " + u.Profile.LastName)
}

// mail.UserProvider.GetEmail
func (u *UserMgo) GetEmail() (email string) {
	email, _ = u.MainEmail()
	return email
}

// mail.UserProvider
func (u *UserMgo) GetName() string {
	return u.Name()
}

// mail.UserProvider
func (u *UserMgo) GetFirstName() string {
	return strings.Title(u.Profile.FirstName)
}

// sms.PhoneNumberProvider
func (u *UserMgo) GetPhoneNumber() string {
	return u.Profile.Telephone
}

func (u *UserMgo) String() string {
	email, _ := u.MainEmail()
	return fmt.Sprintf("[%s <%s> %s]", u.Name(), email, u.ID.Hex())
}

func (u *UserMgo) MainEmail() (email string, err error) {
	if len(u.Emails) == 0 {
		return "", errors.New("no active emails")
	}

	return u.Emails[0].Email, nil
}

func (u *UserMgo) HasPhoto() bool {
	return u.Profile.Avatar != nil
}

func (u *UserMgo) HasAvailability() bool {
	if u.Tutoring != nil {
		return u.Tutoring.Availability != nil
	}

	return false
}

func (u *UserMgo) HasAvailabilityFrom(t time.Time) bool {
	hasAvailability := false
	availability := u.Tutoring.Availability
	if availability == nil {
		return false
	}

	slots := availability.Slots
	recurrent := availability.Recurrent
	if slots == nil && recurrent == nil {
		return false
	}

	if slots != nil {
		for _, slot := range slots {
			if slot.To.After(t) && slot.From.After(t) {
				hasAvailability = true
				break
			}
		}
	}

	// no need for date comparisons
	if recurrent != nil {
		hasAvailability = true
	}

	return hasAvailability
}

func (u *UserMgo) HasDegree(university string, course string) (has bool) {
	query := GetCollection("users").Find(
		bson.M{
			"_id":                         u.ID,
			"tutoring.degrees.university": university,
			"tutoring.degrees.course":     course,
		},
	)

	n, _ := query.Count()

	return n > 0
}

func (u *UserMgo) AddDegree(degree TutoringDegree) (err error) {
	degree.ID = bson.NewObjectId()
	return GetCollection("users").UpdateId(u.ID, bson.M{
		"$push": bson.M{
			"tutoring.degrees": degree,
		},
	})
}

func (u *UserMgo) DeleteDegree(id bson.ObjectId) error {
	var found bool
	for _, d := range u.Tutoring.Degrees {
		if d.ID == id {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("user does not have the specified degree")
	}

	if err := GetCollection("users").UpdateId(u.ID, bson.M{
		"$pull": bson.M{
			"tutoring.degrees": bson.M{
				"_id": id,
			},
		},
	}); err != nil {
		return errors.Wrap(err, "couldn't delete degree from database")
	}

	return nil
}

func (u *UserMgo) AddFavorite(favoriteTutor FavoriteTutor) (err error) {

	favoriteTutor.ID = bson.NewObjectId()

	logger.Get().Info("Favorite tutor: ", favoriteTutor)
	logger.Get().Info("Favorite tutor user: ", u.ID)

	if err := GetCollection("users").UpdateId(u.ID, bson.M{
		"$push": bson.M{
			"favorite.tutors": favoriteTutor,
		},
	}); err != nil {
		logger.Get().Errorf("Add Favorite tutor error: %v", err)
		return errors.Wrap(err, "couldn't add favorite tutor to database")
	}

	favoriteStudent := FavoriteStudent{}
	favoriteStudent.ID = bson.NewObjectId()
	favoriteStudent.Student = u.ID

	logger.Get().Info("Favorite student: ", favoriteStudent)

	if err := GetCollection("users").UpdateId(favoriteTutor.Tutor, bson.M{
		"$push": bson.M{
			"favorite.students": favoriteStudent,
		},
	}); err != nil {
		return errors.Wrap(err, "couldn't add favorite student to database")
	}

	return nil
}

func (u *UserMgo) IsAlreadyFavorite(tutor bson.ObjectId) (has bool) {
	query := GetCollection("users").Find(
		bson.M{
			"_id":                   u.ID,
			"favorite.tutors.tutor": tutor,
		},
	)

	n, _ := query.Count()

	return n > 0
}

func (u *UserMgo) RemoveFavorite(id bson.ObjectId) error {

	if err := GetCollection("users").UpdateId(u.ID, bson.M{
		"$pull": bson.M{
			"favorite.tutors": bson.M{
				"tutor": id,
			},
		},
	}); err != nil {
		return errors.Wrap(err, "couldn't remove favorite tutor from database")
	}

	if err := GetCollection("users").UpdateId(id, bson.M{
		"$pull": bson.M{
			"favorite.students": bson.M{
				"student": u.ID,
			},
		},
	}); err != nil {
		return errors.Wrap(err, "couldn't remove favorite student from database")
	}

	return nil
}

func (u *UserMgo) HasSubject(subject bson.ObjectId) (has bool) {
	query := GetCollection("users").Find(
		bson.M{
			"_id":                       u.ID,
			"tutoring.subjects.subject": subject,
		},
	)

	n, _ := query.Count()

	return n > 0
}

func (u *UserMgo) HasSubjects() (has bool) {
	query := GetCollection("users").Find(
		bson.M{
			"_id":               u.ID,
			"tutoring.subjects": bson.M{"$exists": true, "$not": bson.M{"$size": 0}},
		},
	)

	n, _ := query.Count()

	return n > 0
}

func (u *UserMgo) AddSubject(subject TutoringSubject) (err error) {
	subject.ID = bson.NewObjectId()
	return GetCollection("users").UpdateId(u.ID, bson.M{
		"$push": bson.M{
			"tutoring.subjects": subject,
		},
	})
}

func (u *UserMgo) UpdateSubject(subject TutoringSubject) (err error) {

	err = GetCollection("users").Update(
		bson.M{
			"_id":                   u.ID,
			"tutoring.subjects._id": subject.ID,
		},
		bson.M{
			"$set": bson.M{
				"tutoring.subjects.$.certificate": subject.Certificate,
			},
		},
	)

	if err != nil {
		return fmt.Errorf("couldn't update subject: %s", err)
	}

	return nil
}

func (u *UserMgo) DeleteSubject(id bson.ObjectId) error {
	var found bool
	for _, s := range u.Tutoring.Subjects {
		if s.ID == id {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("user does not have the specified subject")
	}

	if err := GetCollection("users").UpdateId(u.ID, bson.M{
		"$pull": bson.M{
			"tutoring.subjects": bson.M{
				"_id": id,
			},
		},
	}); err != nil {
		return errors.Wrap(err, "couldn't delete subject from database")
	}

	return nil
}

func (u *UserMgo) AddFile(f *Upload) (err error) {
	return GetCollection("users").UpdateId(u.ID, bson.M{
		"$push": bson.M{
			"files": f.ID,
		},
	})
}

func (u *UserMgo) HasFile(f bson.ObjectId) (has bool) {
	query := GetCollection("users").Find(
		bson.M{
			"_id":   u.ID,
			"files": f,
		},
	)
	n, _ := query.Count()
	return n > 0
}

func (u *UserMgo) DeleteFile(id bson.ObjectId) error {
	if err := GetCollection("users").UpdateId(u.ID, bson.M{
		"$pull": bson.M{
			"files": id,
		},
	}); err != nil {
		return errors.Wrap(err, "couldn't delete subject from database")
	}

	return nil
}

func (u *UserDto) GetReviewFrom(reviewer *PublicUserDto) (*UserReviewDto, bool) {
	if u == nil || reviewer == nil {
		return nil, false
	}

	q := GetCollection("reviews").Find(bson.M{"user": u.ID, "reviewer": reviewer.ID})

	n, err := q.Count()
	if err != nil {
		return nil, false
	}

	if n == 0 {
		return nil, false
	}

	review := &UserReviewDto{}
	if err := q.One(&review); err != nil {
		return nil, false
	}

	return review, true
}

func (u *UserMgo) GetReviewFrom(reviewer *UserMgo) (review *UserReviewDto, yes bool) {

	q := GetCollection("reviews").Find(bson.M{
		"user":     u.ID,
		"reviewer": reviewer.ID,
	})

	n, err := q.Count()

	if err != nil {
		logger.Get().Errorf("Error checking if reviewed: %v", err)
		return nil, false
	}

	if n == 0 {
		return nil, false
	}

	if err := q.One(&review); err != nil {
		return nil, false
	}

	yes = true

	return
}

func (u *UserMgo) UpdateRating() (err error) {
	q := GetCollection("reviews").Pipe([]bson.M{
		{
			"$match": bson.M{
				"user": u.ID,
			},
		},
		{
			"$project": bson.M{
				"communication":   1,
				"clarity":         1,
				"professionalism": 1,
				"patience":        1,
				"helpfulness":     1,
			},
		},
	})

	reviews := make([]UserReview, 0)

	if err := q.All(&reviews); err != nil {
		return err
	}

	meanRating := make([]float64, len(reviews))

	for i, review := range reviews {
		meanRating[i] = (review.Communication + review.Clarity + review.Professionalism + review.Patience + review.Helpfulness) / 5
	}

	var sum float64
	for _, n := range meanRating {
		sum += n
	}

	return GetCollection("users").UpdateId(
		u.ID,
		bson.M{
			"$set": bson.M{
				"tutoring.rating":    sum / float64(len(meanRating)),
				"tutoring.reviewers": len(reviews),
			},
		},
	)
}

type passType string
type authScope string

const (
	PasswordBcrypt passType = "bcrypt"
)

const (
	AuthScopeAuth                  authScope = "auth"
	AuthScopeForgotPassword        authScope = "forgot-password"
	AuthScopeVerifyEmail           authScope = "verify-email"
	AuthScopeVerifyAccount         authScope = "verify-account"
	AuthScopeCompleteAccount       authScope = "complete-account"
	AuthScopeResendActivationEmail authScope = "resend-activation-email"
)

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in"`
	Expires      int64  `json:"expires"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope,omitempty"`
}

func (u *UserMgo) GetAuthenticationToken(scopeName authScope) (token *TokenResponse, err error) {
	iat := time.Now()
	eat := time.Now()

	eat = eat.Add(time.Hour * 24)

	issued := jose.Header("iat", iat.Unix())
	expire := jose.Header("eat", eat.Unix())
	secret := jose.Header("secret", u.Services.Secret)
	scope := jose.Header("scope", string(scopeName))

	accessToken, err := jose.Sign(
		u.ID.Hex(),
		jose.HS256,
		[]byte(config.GetConfig().GetString("security.token")),
		issued,
		expire,
		scope,
		secret,
	)

	token = &TokenResponse{
		AccessToken: accessToken,
		Expires:     eat.Unix(),
		ExpiresIn:   eat.Unix() - time.Now().Unix(),
		TokenType:   "Bearer",
		Scope:       string(scopeName),
	}

	return token, err
}

func (u *UserMgo) UpdatePassword(password string, kind passType) (err error) {
	logger.Get().Infof("updating with password: %s", password)
	secret := make([]byte, 16)
	rand.Read(secret)

	logger.Get().Infof("updating with secret: %s", string(secret))

	bcryptHash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)

	logger.Get().Infof("bcryptHash: %s", string(bcryptHash))

	path := fmt.Sprintf("services.password.%s", string(kind))

	return GetCollection("users").UpdateId(u.ID, bson.M{
		"$set": bson.M{
			path:              string(bcryptHash),
			"services.secret": string(secret),
		},
	})
}

func (u *UserMgo) HasPassword(password string, kind passType) (yes bool) {

	if err := bcrypt.CompareHashAndPassword(
		[]byte(u.Services.Password.Bcrypt),
		[]byte(password),
	); err == nil {
		return true
	}

	return
}

func (u *UserMgo) UpdateTimezone(tz string) (err error) {
	if err = GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{"timezone": tz}}); err != nil {
		return errors.Wrap(err, "couldn't update user timezone")
	}
	u.Timezone = tz
	return nil
}

func (u *UserMgo) UpdateTutoring(t *Tutoring) error {
	if t.Rate < 30 {
		return fmt.Errorf("rate can't be lower than $30")
	}

	if t.LessonBuffer < 15 {
		return fmt.Errorf("lesson buffer time can't be lower than 15 minutes")
	}

	if t.Title == "" {
		return fmt.Errorf("tutoring title is required")
	}

	if len(t.Title) < 10 || len(t.Title) > 100 {
		return fmt.Errorf("tutoring title must be between 10 and 100 characters")
	}

	u.Tutoring.Rate = t.Rate
	u.Tutoring.LessonBuffer = t.LessonBuffer
	u.Tutoring.Title = t.Title
	u.Tutoring.Meet = t.Meet

	err := GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{"tutoring": u.Tutoring}})
	if err != nil {
		return fmt.Errorf("couldn't save tutoring to database: %s", err)
	}

	return nil
}

func (u *UserMgo) UpdateLocation(newLoc *UserLocation) error {
	if newLoc.State == "" {
		return fmt.Errorf("state is required")
	}

	if newLoc.City == "" {
		return fmt.Errorf("city is required")
	}

	if newLoc.Address == "" {
		return fmt.Errorf("address is required")
	}

	if newLoc.PostalCode == "" {
		return fmt.Errorf("postal is required")
	}

	u.Location = newLoc

	err := GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{"location": u.Location}})
	if err != nil {
		return fmt.Errorf("couldn't save location to database: %s", err)
	}

	return nil
}

func (u *UserMgo) UpdatePhone(p *Profile) error {
	if p.Telephone == "" {
		return fmt.Errorf("phone number is required")
	} else {
		phone, err := phonenumbers.Parse(p.Telephone, "US")
		if err != nil || !phonenumbers.IsValidNumber(phone) {
			return fmt.Errorf("phone number is invalid")
		}
	}

	u.Profile.Telephone = p.Telephone

	if u.Preferences != nil {
		if !u.Preferences.ReceiveSMSUpdates {
			u.Preferences.ReceiveSMSUpdates = true
		}
	}

	err := GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{
		"profile.telephone": u.Profile.Telephone,
	}})

	if err != nil {
		return fmt.Errorf("couldn't save profile to database: %s", err)
	}

	return nil
}

func (u *UserMgo) UpdateProfile(p *Profile) error {
	if p.FirstName == "" {
		return fmt.Errorf("first name is required")
	}
	u.Profile.FirstName = p.FirstName

	if p.LastName == "" {
		return fmt.Errorf("last name is required")
	}
	u.Profile.LastName = p.LastName

	if u.IsTutor() && p.About == "" {
		return fmt.Errorf("about is required")
	} else if u.IsTutor() && p.About != "" {
		u.Profile.About = p.About
	}

	if p.Telephone == "" {
		return fmt.Errorf("phone number is required")
	} else {
		phone, err := phonenumbers.Parse(p.Telephone, "US")
		if err != nil || !phonenumbers.IsValidNumber(phone) {
			return fmt.Errorf("phone number is invalid")
		}
	}
	u.Profile.Telephone = p.Telephone

	if p.Birthday.IsZero() {
		return fmt.Errorf("date of birth is required")
	}
	u.Profile.Birthday = p.Birthday

	if p.EmployerIdentificationNumber != "" {
		u.Profile.EmployerIdentificationNumber = p.EmployerIdentificationNumber
	}

	if p.SocialSecurityNumber != "" {
		u.Profile.SocialSecurityNumber = p.SocialSecurityNumber
	}

	err := GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{"profile": u.Profile}})
	if err != nil {
		return fmt.Errorf("couldn't save profile to database: %s", err)
	}

	return nil
}

func (u *UserMgo) UpdatePayoutData(ein, ssn string) error {
	if !utils.AreMutualExclusive(ein, ssn) {
		return fmt.Errorf("EIN and SSN fields are mutual exclusive")
	}

	u.Profile.EmployerIdentificationNumber = ein
	u.Profile.SocialSecurityNumber = fmt.Sprintf("%s%s", "###-##-", ssn[len(ssn)-4:])

	err := GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{
		"profile.employer_identification_number": u.Profile.EmployerIdentificationNumber,
		"profile.social_security_number":         u.Profile.SocialSecurityNumber,
	}})
	if err != nil {
		return fmt.Errorf("couldn't save payout data to database: %s", err)
	}

	return nil
}

func (u *UserMgo) SetAvatar(upload *Upload) (err error) {
	u.Profile.Avatar = upload
	return GetCollection("users").UpdateId(u.ID, bson.M{
		"$set": bson.M{
			"profile.avatar": u.Profile.Avatar,
		},
	})
}

func (u *UserMgo) SetProfileChecked(t *time.Time) (err error) {
	u.Tutoring.ProfileChecked = t
	return GetCollection("users").UpdateId(u.ID, bson.M{
		"$set": bson.M{
			"tutoring.profile_checked": t,
		},
	})
}

func (u *UserMgo) GetLessonsSpread(from, to time.Time) (lessons []LessonDto) {
	defer func() {
		if r := recover(); r != nil {
			logger.Get().Errorf("recovered panic in GetLessonSpread: %v", r)
			debug.PrintStack()
		}
	}()
	lessons = make([]LessonDto, 0)
	queried := make([]LessonMgo, 0)

	if u.IsAffiliate() {
		return lessons
	}

	query := bson.M{
		"$and": []bson.M{
			{"$or": []bson.M{
				{"tutor": u.ID},
				{"students": u.ID},
			}},
			{"starts_at": bson.M{"$gte": from}},
			{"ends_at": bson.M{"$lte": to}},
		},
	}

	GetCollection("lessons").Find(query).All(&queried)

	for _, lesson := range queried {
		logger.Get().Infof("lesson: %#v", lesson)
		logger.Get().Infof("final end date: %s", lesson.GetFinalLessonEndsAt())

		// smahr 1/13/21 continues here so we do not run into next condition(s).
		// This will check if this is an individual lesson of a recurring lesson group and add the lesson to the list
		// this keeps new recurrent lessons backward compatible
		if lesson.RecurrentID != nil && timeline.SlotEnters(lesson.GetTimelineSlot(), from, to) {
			dto, _ := lesson.DTO()
			lessons = append(lessons, *dto)
			continue
		}

		if !lesson.Recurrent && timeline.SlotEnters(lesson.GetTimelineSlot(), from, to) {
			dto, _ := lesson.DTO()
			lessons = append(lessons, *dto)
		}

		if lesson.Recurrent {
			tmpSlot := lesson.GetTimelineSlot()
			lessonsAdded := 0
			for tmpSlot != nil {
				// exits early if the final lesson end time is before current slot's to time
				if tmpSlot.GetTo().After(lesson.GetFinalLessonEndsAt()) {
					break
				}

				if timeline.SlotIn(tmpSlot, from, to) {
					lessonCopy := lesson.Clone()
					lessonCopy.StartsAt = tmpSlot.GetFrom()
					lessonCopy.EndsAt = tmpSlot.GetTo()
					dto, _ := lessonCopy.DTO()
					lessons = append(lessons, *dto)
					lessonsAdded += 1
				}

				// if recurrent count is met, exit loop
				if lesson.RecurrentCount >= 1 && lessonsAdded == lesson.RecurrentCount {
					break
				}

				tmpSlot = timeline.Shift(tmpSlot, from, to)
			}
		}
	}

	return lessons
}

func (u *UserMgo) GetLessonsDates(from, to time.Time) []time.Time {
	queried := make([]LessonMgo, 0)

	var userEntity string
	if u.IsTutor() {
		userEntity = "tutor"
	} else if u.IsStudent() {
		userEntity = "students"
	} else {
		return []time.Time{}
	}

	query := bson.M{
		"$and": []bson.M{
			{userEntity: u.ID},
			{"starts_at": bson.M{"$gte": from}, "ends_at": bson.M{"$lte": to}},
		},
	}

	_ = GetCollection("lessons").Find(query).All(&queried)

	var dates []time.Time
	for _, lesson := range queried {
		// smahr 1/13/21 continues here so we do not run into next condition(s).
		// This will check if this is an individual lesson of a recurring lesson group and add the lesson to the list
		// this keeps new recurrent lessons backward compatible
		if lesson.RecurrentID != nil && timeline.SlotEnters(lesson.GetTimelineSlot(), from, to) {
			dto, _ := lesson.DTO()
			dates = append(dates, dto.StartsAt)
			continue
		}

		if !lesson.Recurrent && timeline.SlotEnters(lesson.GetTimelineSlot(), from, to) {
			dto, _ := lesson.DTO()
			dates = append(dates, dto.StartsAt)
		}

		if lesson.Recurrent {
			tmpSlot := lesson.GetTimelineSlot()

			for tmpSlot != nil {
				if timeline.SlotEnters(tmpSlot, from, to) {
					lessonCopy := lesson.Clone()
					lessonCopy.StartsAt = tmpSlot.GetFrom()
					lessonCopy.EndsAt = tmpSlot.GetTo()
					dto, _ := lessonCopy.DTO()

					dates = append(dates, dto.StartsAt)
				}
				tmpSlot = timeline.Shift(tmpSlot, from, to)
			}
		}
	}

	deduplicatedDates := uniqueDaysInMonth(dates)

	sort.Slice(deduplicatedDates, func(i, j int) bool {
		return deduplicatedDates[i].Before(deduplicatedDates[j])
	})

	return deduplicatedDates
}

func (u *UserMgo) GetLessonBetween(from, to time.Time) *LessonDto {
	var userEntity string
	if u.IsTutor() {
		userEntity = "tutor"
	} else if u.IsStudent() {
		userEntity = "students"
	} else {
		return nil
	}
	query := GetCollection("lessons").Find(bson.M{
		"$and": []bson.M{
			{userEntity: u.ID.Hex()},
			{"starts_at": bson.M{"$gte": from, "$lte": to}},
		},
	})
	var lesson LessonDto
	if err := query.One(&lesson); err != nil {
		return nil
	}
	return &lesson
}

func uniqueDaysInMonth(dates []time.Time) []time.Time {
	keys := make(map[int]bool)
	var list []time.Time
	for _, entry := range dates {
		day := entry.Day()
		if _, value := keys[day]; !value {
			keys[day] = true
			list = append(list, entry)
		}
	}
	return list
}

func (u *UserMgo) GetLessonsTimelineSlots() (slots []timeline.SlotProvider) {

	queried := make([]LessonMgo, 0)

	query := bson.M{
		"$and": []bson.M{
			{"$or": []bson.M{
				{"tutor": u.ID},
				{"students": u.ID},
			}},
			{
				"$or": []bson.M{
					{"recurrent": false, "starts_at": bson.M{"$gte": time.Now()}},
					{"recurrent": true},
				},
			},
			{"state": 1},
		},
	}

	GetCollection("lessons").Find(query).All(&queried)

	for _, lesson := range queried {
		slots = append(slots, lesson.GetTimelineSlot())
	}

	return slots
}

func (u *UserMgo) GetLessons(tutorOnly bool) (lessons []LessonMgo) {

	var condition = []bson.M{
		{"tutor": u.ID},
		{"students": u.ID},
	}

	if tutorOnly {
		condition = []bson.M{
			{"tutor": u.ID},
		}
	}

	query := bson.M{
		"$or": condition,
	}

	GetCollection("lessons").Find(query).All(&lessons)

	return
}

func (u *UserMgo) GetInstantSessions(tutorOnly ...bool) ([]*LessonMgo, error) {

	var condition = []bson.M{
		{"tutor": u.ID},
		{"student": u.ID},
	}

	if len(tutorOnly) > 0 && tutorOnly[0] == true {
		condition = []bson.M{
			{"tutor": u.ID},
		}
	}

	var sessions []*LessonMgo
	err := GetCollection("lessons").Find(bson.M{"$or": condition}).All(&sessions)

	return sessions, err
}

func (u *UserMgo) SetPresence(p UserPresence) (err error) {
	u.Online = p
	return GetCollection("users").Update(
		bson.M{
			"_id": u.ID,
		},
		bson.M{
			"$set": bson.M{
				"online": p,
			},
		},
	)
}

func (u *UserMgo) HasEmail(email string) (yes bool) {
	for _, u := range u.Emails {
		if u.Email == email {
			return true
		}
	}
	return false
}

func (u *UserMgo) VerifyEmail(email string) (err error) {
	return
}

func (u *UserMgo) HasRole(role Role) (yes bool) {
	return u.Role&role != 0
}

func (u *UserMgo) AddEmail(email string) (err error) {
	u.Emails = append(u.Emails, RegisteredEmail{
		Email:   email,
		Created: time.Now(),
	})

	return GetCollection("users").Update(
		bson.M{
			"_id": u.ID,
		},
		bson.M{
			"$set": bson.M{
				"emails": u.Emails,
			},
		},
	)
}

func (u *UserMgo) IsTutor() bool {
	if u.Tutoring != nil && u.HasRole(RoleTutor) {
		return true
	}
	return false
}

func (u *UserMgo) IsTestTutor() bool {
	if u.Tutoring != nil && u.HasRole(RoleTutor) && u.IsTestAccount {
		return true
	}
	return false
}

func (u *UserMgo) IsTutorStrict() bool {
	return u.Tutoring != nil && u.Role == RoleTutor
}

func (u *UserMgo) IsStudent() bool {
	return u.HasRole(RoleStudent)
}

func (u *UserMgo) IsAdmin() bool {
	return u.HasRole(RoleAdmin)
}

func (u *UserMgo) IsTestStudent() bool {
	return u.HasRole(RoleStudent) && u.IsTestAccount
}

func (u *UserMgo) IsStudentStrict() bool {
	return u.Role == RoleStudent
}

func (u *UserMgo) IsAffiliate() bool {
	return u.Role == RoleAffiliate
}

func (u *UserMgo) RoleForLesson(lesson *LessonMgo) (role Role, found bool) {
	if lesson.Tutor.Hex() == u.ID.Hex() {
		return RoleTutor, true
	}

	for _, student := range lesson.Students {
		if student.Hex() == u.ID.Hex() {
			return RoleStudent, true
		}
	}

	return 0, false
}

func (u *UserMgo) IsFree(when time.Time, duration time.Duration) bool {
	ends := when.Add(duration)

	query := bson.M{"$and": []bson.M{
		{"$or": []bson.M{
			{"tutor": u.ID},
			{"student": u.ID},
		}},
		{"$or": []bson.M{
			{"starts_at": bson.M{"$lt": when}, "ends_at": bson.M{"$gt": ends}},
			{"starts_at": bson.M{"$gt": when, "$lt": ends}},
			{"ends_at": bson.M{"$gt": when, "$lt": ends}},
		}},
		{"$and": []bson.M{
			{"state": bson.M{"$ne": LessonCancelled}},
		}},
	}}

	n, err := GetCollection("lessons").Find(query).Count()
	if err != nil {
		return false
	}

	if n == 0 {
		return true
	}

	return false
}

func (u *UserMgo) IsFreeIgnoreIDs(when time.Time, duration time.Duration, IDs []bson.ObjectId) bool {
	ends := when.Add(duration)

	query := GetCollection("lessons").Find(bson.M{"$and": []bson.M{
		{"$or": []bson.M{
			{"tutor": u.ID},
			{"student": u.ID},
		}},
		{"$or": []bson.M{
			{"starts_at": bson.M{"$gte": when, "$lte": ends}},
			{"ends_at": bson.M{"$gte": when, "$lte": ends}},
			{"starts_at": bson.M{"$lte": when}, "ends_at": bson.M{"$gte": ends}},
		}},
		{"$and": []bson.M{
			{"state": bson.M{"$ne": LessonCancelled}},
			{"_id": bson.M{"$nin": IDs}},
		}},
	}})

	n, err := query.Count()
	if err != nil {
		return false
	}

	if n == 0 {
		return true
	}

	return false
}

// SetReferralCode generates a pseudo-random string of specified length and assigns it to the user.
// The only way it could fail is by trying too many combinations, meaning that we have too many users
// for n-chars codes. If dbSet is set, it will update the database entry too.
func (u *UserMgo) SetReferralCode(dbSet bool) error {
	var referralCode string
	i := 0
	for {
		// pseudo while loop; if we generate an already-assigned code, try again
		// 100 users with already generated code is a fault on our side
		if i > 100 {
			return errors.New("can't generate a random referral code: infinite loop")
		}

		referralCode = utils.RandString(8)

		// if a user does NOT exist using the referral code, means it's good to use - break the loop
		var referralUser *UserMgo
		GetCollection("users").Find(bson.M{"refer.referral_code": referralCode}).One(&referralUser)
		if referralUser == nil {
			break
		}

		i++
	}

	u.Refer.ReferralCode = referralCode

	if dbSet {
		return GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{"refer.referral_code": referralCode}})
	}

	return nil
}

// SetReferrer sets the referrer on the specified user. Checks if an user exists using the
// specified referral code, and only sets it if referrer exists. Updates referrer on the current
// user and appends the referral code to the referrer's account.
//
// Returns if referrer was set, indicating that credit is due, and an error if needed, being the reason.
func (u *UserMgo) SetReferrer(s string) (*ReferLink, error) {
	if s == "" {
		return nil, errors.New("empty referrer")
	}

	// populate the referrer field only if there's an user with that code
	var referrerUser *UserMgo
	if err := GetCollection("users").Find(bson.M{"refer.referral_code": s}).One(&referrerUser); err != nil {
		return nil, errors.Wrap(err, "couldn't find user")
	}

	if referrerUser == nil {
		return nil, errors.New("found user is nil")
	}

	// update the referrer on the current user
	u.Refer.Referrer = s
	if err := GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{"refer.referrer": s}}); err != nil {
		return nil, errors.Wrap(err, "couldn't set referrer to user")
	}

	var referLink *ReferLink
	GetCollection("refers").Find(bson.M{
		"referrer": referrerUser.ID,
		"email":    u.Username,
	}).Sort("-created_at").One(&referLink)

	if referLink == nil {
		// no refer link, so no invitation was sent
		referLink = &ReferLink{
			Referrer:  &referrerUser.ID,
			Referral:  &u.ID,
			Step:      SignedUpStep,
			Affiliate: referrerUser.IsAffiliate(),
		}

		if err := referLink.Insert(); err != nil {
			return nil, errors.Wrap(err, "couldn't insert refer link")
		}

		return referLink, nil
	}

	// we have refer link, invitation was sent
	if err := referLink.SetReferral(&u.ID); err != nil {
		return referLink, errors.Wrap(err, "couldn't set referral on refer link")
	}

	if err := referLink.SetStep(SignedUpStep); err != nil {
		return referLink, errors.Wrap(err, "couldn't set step on refer link")
	}

	return referLink, nil
}

func (u *UserMgo) TimezoneLocation() *time.Location {
	if l, err := time.LoadLocation(u.Timezone); err == nil {
		return l
	}
	return time.UTC
}

func (u *UserMgo) GetFiles() ([]*FilesMgo, error) {
	files := make([]*FilesMgo, len(u.Files))
	if err := GetCollection("files").FindId(bson.M{"$in": u.Files}).All(&files); err != nil {
		return nil, errors.Wrap(err, "couldn't find files")
	}

	return files, nil
}

func (to *UserPreferences) Copy(from map[string]interface{}) (err error) {
	return utils.CopyStructMatch(to, from)
}

func (to *Tutoring) Copy(from map[string]interface{}) (err error) {
	return utils.CopyStructMatch(to, from)
}

func (to *Profile) Copy(from map[string]interface{}) (err error) {
	return utils.CopyStructMatch(to, from)
}

func (ul *UserLocation) Copy(from map[string]interface{}) (err error) {
	return utils.CopyStructMatch(ul, from)
}

type (
	// ProfilePublic is a recution of Profie to hide sensitive data
	ProfilePublic struct {
		FirstName string  `json:"first_name" bson:"first_name"`
		LastName  string  `json:"last_name" bson:"last_name"`
		About     string  `json:"about,omitempty" bson:"about,omitempty"`
		Avatar    *Upload `json:"avatar" bson:"avatar,omitempty"`
	}

	// UserPublic is a reduction of UserDto to hide sensitve data
	UserPublic struct {
		ID       bson.ObjectId `json:"_id" bson:"_id"`
		Profile  ProfilePublic `json:"profile" bson:"profile"`
		Location *UserLocation `json:"location,omitempty" bson:"location"`
		Tutoring *TutoringDto  `json:"tutoring,omitempty" bson:"tutoring"`
		Timezone string        `json:"timezone,omitempty" bson:"timezone"`
		Online   UserPresence  `json:"online" bson:"online"`
	}
)

// ToPublic will hide sensitive data of a user to be able to us in things like search
func (u *UserDto) ToPublic() *UserPublic {
	return &UserPublic{
		ID: u.ID,
		Profile: ProfilePublic{
			FirstName: u.Profile.FirstName,
			LastName:  u.Profile.LastName,
			About:     u.Profile.About,
			Avatar:    u.Profile.Avatar,
		},
		Location: u.Location,
		Tutoring: u.Tutoring,
		Timezone: u.Timezone,
		Online:   u.Online,
	}
}
