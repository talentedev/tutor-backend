package messaging

type P map[string]string

type Tpl string

const (
	TPL_RECOVER_PASSWORD      Tpl = "recover-password"
	TPL_VERIFY_EMAIL          Tpl = "verify-email"
	TPL_REVIEWING_APPLICATION Tpl = "we-are-reviewing-your-application"
	TPL_AFFILIATE_WELCOME     Tpl = "affiliate-welcome-to-learnt"

	// STUDENT_NAME, DASHBOARD_URL
	TPL_STUDENT_WELCOME                    Tpl = "student-welcome-to-tutor-app"
	TPL_AFFILIATE_USER_SIGNED_UP           Tpl = "affiliate-user-signed-up"
	TPL_TUTOR_APPLICATION_APPROVED         Tpl = "tutor-application-approved"
	TPL_TUTOR_APPLICATION_REJECTED         Tpl = "tutor-rejected"
	TPL_TUTOR_PROPOSED_LESSON              Tpl = "tutor-proposed-lesson"
	TPL_STUDENT_PROPOSED_LESSON            Tpl = "student-proposed-lesson"
	TPL_TUTOR_PROPOSED_LESSON_CHANGE       Tpl = "tutor-proposed-lesson-change"
	TPL_STUDENT_PROPOSED_LESSON_CHANGE     Tpl = "student-proposed-lesson-change" // no sms
	TPL_AFFILIATE_PAYMENT_PENDING          Tpl = "affiliate-payment-pending"
	TPL_AFFILIATE_PAYMENT_NO_BANK_ACCOUNT  Tpl = "affiliate-payment-no-bank-account"
	TPL_JOIN_INVITATION                    Tpl = "you-ve-been-invited-to-join-ta"
	TPL_LESSON_NOTE_CREATED                Tpl = "lesson-note-created"
	TPL_LESSON_STARTING                    Tpl = "lesson-starting" // no sms
	TPL_FLAG_MESSAGE                       Tpl = "flag-message"    // no sms
	TPL_NEW_APPLICATION_SUBMITTED          Tpl = "new-application-submitted"
	TPL_BACKGROUND_CHECK_COMPLETE          Tpl = "background-check-complete"
	TPL_CANCELLED_GREATER_THAN_24_HOURS    Tpl = "student-cancelled-lesson-with-tutor"
	TPL_CANCELLED_WITHIN_24_HOURS          Tpl = "student-cancels-within-24-hours"
	TPL_TUTOR_REVIEW                       Tpl = "admin-review-notification"
	TPL_TUTOR_CANCELLED                    Tpl = "tutor-cancels"
	TPL_TUTOR_CANCELLED_WITHIN_24_HOURS    Tpl = "tutor-cancels-within-24-hours"
	TPL_SUBJECT_EDUCATION_FOR_VERIFICATION Tpl = "subject-education-submitted-for-verification"
	TPL_LESSON_REMINDER                    Tpl = "lesson-reminder"
	TPL_LESSON_NO_SHOW_TUTOR               Tpl = "lesson-noti-tutor-no-show-tutor"
	TPL_LESSON_NO_SHOW_STUDENT             Tpl = "lesson-noti-tutor-no-show-student"
	TPL_LESSON_NO_SHOW_ADMIN               Tpl = "lesson-noti-tutor-no-show-admin"
	TPL_FIVE_MINUTES_LATE                  Tpl = "you-re-late-and-first-name-is-waiting"
	TPL_SCHEDULED_LESSON_IS_STARTING       Tpl = "your-scheduled-lesson-is-starting"
	TPL_TUTOR_ONLINE_NOW                   Tpl = "tutor-online-now"
	TPL_INCOMPLETE_PROFILE                 Tpl = "your-profile-is-incomplete"
	TPL_LESSON_REMINDER_7AM                Tpl = "lesson-7-am-noti"
	TPL_LESSON_REMINDER_2_HOURS_PRIOR      Tpl = "lesson-noti-2-hours-prior"
	TPL_LESSON_REMINDER_15_MINS_PRIOR      Tpl = "lesson-noti-15-mins-prior"
	TPL_MESSAGE_NOTIFICATION               Tpl = "message-notification"
	TPL_INSTANT_LESSON_REQUEST      	   Tpl = "instant-session-requested"

	HIRING_EMAIL = "hello@learnt.io"
)

type UserProvider interface {
	To() string
	GetFirstName() string
}

type Sender interface {
	Send(u UserProvider, template Tpl, params *P) error
	SendTo(to string, template Tpl, params *P) error
}
