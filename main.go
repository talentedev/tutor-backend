package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/jobs"
	"gitlab.com/learnt/api/pkg/logger"
	notifs "gitlab.com/learnt/api/pkg/notifications"
	"gitlab.com/learnt/api/pkg/routes/auth"
	"gitlab.com/learnt/api/pkg/routes/bgcheck"
	"gitlab.com/learnt/api/pkg/routes/checkr"
	"gitlab.com/learnt/api/pkg/routes/common"
	"gitlab.com/learnt/api/pkg/routes/countries"
	"gitlab.com/learnt/api/pkg/routes/hooks"
	"gitlab.com/learnt/api/pkg/routes/importcontacts"
	"gitlab.com/learnt/api/pkg/routes/intercom"
	"gitlab.com/learnt/api/pkg/routes/lessons"
	"gitlab.com/learnt/api/pkg/routes/me"
	"gitlab.com/learnt/api/pkg/routes/messenger"
	"gitlab.com/learnt/api/pkg/routes/metrics"
	"gitlab.com/learnt/api/pkg/routes/notifications"
	"gitlab.com/learnt/api/pkg/routes/oidc"
	"gitlab.com/learnt/api/pkg/routes/payments"
	"gitlab.com/learnt/api/pkg/routes/platform"
	"gitlab.com/learnt/api/pkg/routes/proxy"
	"gitlab.com/learnt/api/pkg/routes/refer"
	"gitlab.com/learnt/api/pkg/routes/register"
	"gitlab.com/learnt/api/pkg/routes/reviews"
	"gitlab.com/learnt/api/pkg/routes/search"
	"gitlab.com/learnt/api/pkg/routes/subjects"
	"gitlab.com/learnt/api/pkg/routes/surveys"
	"gitlab.com/learnt/api/pkg/routes/universities"
	"gitlab.com/learnt/api/pkg/routes/uploads"
	"gitlab.com/learnt/api/pkg/routes/users"
	"gitlab.com/learnt/api/pkg/routes/vcr"
	verify_account "gitlab.com/learnt/api/pkg/routes/verify-account"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/utils/messaging"
	"gitlab.com/learnt/api/pkg/ws"
	gintrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gin-gonic/gin"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

var Build string   // Updated by learnt-cli
var Version string // Updated by learnt-cli

var wg *sync.WaitGroup
var signals chan os.Signal

func init() {
	os.Setenv("TZ", "UTC")
	wg = &sync.WaitGroup{}
	signals = make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
}

func routes(router *gin.Engine, close func()) {
	store := cookie.NewStore([]byte(config.GetConfig().GetString("security.token")))
	router.Use(sessions.Sessions("learnt", store))
	router.Use(core.CORS)

	// router.Use(ddgin.Middleware("api"))
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, core.NewErrorResponse("Route not found"))
	})

	ctx, cancel := context.WithCancel(context.WithValue(context.TODO(), "wg", wg))

	ws.Init(router.Group("/ws"))

	auth.Setup(router.Group("/auth"))
	bgcheck.Setup(router.Group("/bgcheck"))
	checkr.Setup(router.Group("/checkr")) //DEPRECATED, will be removed eventually
	common.Setup(router)
	common.SetupCore(router.Group("/url"))
	countries.Setup(router.Group("/countries", core.CORS))
	hooks.Setup(router.Group("/hooks"))
	importcontacts.Setup(router.Group("/import", auth.Middleware, core.CORS))
	intercom.Setup(router.Group("/intercom"))
	lessons.SetupLessons(ctx, router.Group("/lessons", auth.MiddlewareSilent, core.CORS))
	me.Setup(router.Group("/me", auth.Middleware, core.CORS))
	messenger.Setup(router.Group("/messenger", auth.Middleware, core.CORS))
	metrics.Setup(router.Group("/metrics", auth.Middleware, core.CORS))
	notifications.Setup(router.Group("/notifications"))
	notifs.InitNotifications()
	oidc.Setup(router.Group("/oidc", auth.MiddlewareSilent, core.CORS))
	payments.Setup(ctx, router.Group("/payments", auth.Middleware, core.CORS))
	platform.Setup(router.Group("/platform"), Version, Build)
	proxy.Setup(router.Group("/proxy"))
	refer.Setup(router.Group("/refer"))
	register.Setup(router.Group("/register"))
	reviews.Setup(router.Group("/reviews"))
	search.Setup(router.Group("/search"))
	services.InitMessenger(ctx)
	services.InitVCR(ctx)
	subjects.Setup(router.Group("/subjects"))
	surveys.Setup(router.Group("/surveys", core.CORS))
	universities.Setup(router.Group("/universities"))
	uploads.Setup(router.Group("/uploads"))
	users.Setup(router.Group("/users"))
	vcr.Setup(router.Group("/vcr", auth.Middleware, core.CORS))
	verify_account.Setup(router.Group("/verify-account"))

	<-signals

	cancel()
	wg.Wait()
	close()

	fmt.Println("Server is shutdown")
}

func main() {

	var configFile = flag.String("config", "", "path to configuration file")
	flag.Parse()

	runtime.GOMAXPROCS(runtime.NumCPU())

	var conf *config.Config
	var err error
	if *configFile == "" {
		// if no configuration file was passed, use the default config from init
		conf = config.GetConfig()
	} else {
		// If a config file was passed, reload the config with the given configuration
		conf, err = config.LoadConfig(*configFile)
		if err != nil {
			log.Fatalf("failed to load configuration: %v", err)
		}
	}

	tracer.Start(
		tracer.WithAnalytics(true),
		tracer.WithEnv(config.GetConfig().App.Env),
	)
	defer tracer.Stop()

	debugMode := conf.GetString("app.debug") == "true"
	logLevel := logger.INFO
	if debugMode {
		logLevel = logger.DEBUG
	}

	filename := conf.GetString("logfile")
	if filename == "" {
		//Set a reasonable default if there is no log file defined
		filename = "/tmp/api.log"
	}

	l, err := logger.Init(filename, logLevel, config.GetConfig().App.Env)
	if err != nil {
		log.Fatalf("failed to start logger: %v", err)
	}
	defer l.Close()

	log.SetOutput(l.GetFileHandle()) //Make sure all log output is redirected to the main log file

	store.Init()

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	router.Use(gintrace.Middleware("learnt-api"))

	listen := conf.GetString("app.listen")

	jobsEnabled := conf.GetString("app.jobs") == "true"
	if jobsEnabled {
		startCronJobs()
	}

	srv := &http.Server{
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      router,
		Addr:         listen,
	}

	go routes(router, func() {
		srv.Close()
	})

	logger.Get().Infof("Server is listening at %s\n", listen)

	if err := srv.ListenAndServe(); err != nil {
		panic(err)
	}
}

func startCronJobs() {
	c := cron.New()
	// email reminder at 15 minutes and 2 hours before lesson
	_, err := c.AddFunc("*/15 * * * *", func() {
		logger.Get().Infof("running 15 & 120 minute reminder")
		lessonReminder := jobs.UpcomingLessonReminder{
			Pairs: []jobs.TemplateNotificationPair{
				{Duration: 15 * time.Minute, Template: messaging.TPL_LESSON_REMINDER_15_MINS_PRIOR},
				{Duration: 120 * time.Minute, Template: messaging.TPL_LESSON_REMINDER_2_HOURS_PRIOR},
			},
		}
		lessonReminder.RemindUpcomingLesson()
	})

	if err != nil {
		logger.Get().Fatal(err)
	}

	// email reminder every hour to send 7am emails
	_, err = c.AddFunc("0 * * * *", func() {
		logger.Get().Infof("running daily reminder")
		lessonReminder := jobs.DailyLessonReminder{NotifyTimes: []time.Duration{17 * time.Hour}}
		lessonReminder.DailyReminder()
	})

	if err != nil {
		logger.Get().Fatal(err)
	}

	// email reminder at 2 minutes before lesson (checks every minute)
	_, err = c.AddFunc("* * * * *", func() {
		logger.Get().Infof("running 2 minute reminder")
		lessonReminder := jobs.UpcomingLessonReminderInTwoMinutes{NotifyTimes: []time.Duration{2 * time.Minute}}
		lessonReminder.RemindUpcomingLesson()
	})

	if err != nil {
		logger.Get().Fatal(err)
	}

	_, err = c.AddFunc("0 0 * * MON", func() {
		logger.Get().Infof("running weekly reminder")
		reminder := jobs.WeeklyProfileReminder{}
		reminder.WeeklyProfileReminder()
	})

	if err != nil {
		logger.Get().Fatal(err)
	}

	c.Start()
}
