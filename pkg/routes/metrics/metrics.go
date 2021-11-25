package metrics

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/pkg/routes/auth"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/utils"
	"net/http"
	"time"
)

type sessionsMetricResponse struct {
	Data []XYPair `json:"data"`
}

type XYPair struct {
	X string `json:"x"`
	Y int64  `json:"y"`
}

func sessionsMetricHandler(c *gin.Context) {
	auth.IsAdminMiddleware(c)
	res := sessionsMetricResponse{}
	l := services.GetLessons()
	lessons := l.GetDefaultLessons()

	temp := make(map[string]int64)
	tempIdx := make([]string, 0)
	now := time.Now()
	for _, lesson := range lessons {
		if utils.DateIsBefore(lesson.StartsAt.Add(1*time.Hour), now) {
			if _, ok := temp[lesson.StartsAt.Format("2006/01/02")]; !ok {
				tempIdx = append(tempIdx, lesson.StartsAt.Format("2006/01/02"))
			}
			temp[lesson.StartsAt.Format("2006/01/02")]++
		}
	}
	res.Data = make([]XYPair, len(tempIdx))
	for i, key := range tempIdx {
		res.Data[i] = XYPair{X: key, Y: temp[key]}
	}

	c.JSON(http.StatusOK, res)
}

func sessionsHourlyMetricHandler(c *gin.Context) {
	auth.IsAdminMiddleware(c)
	res := sessionsMetricResponse{}
	l := services.GetLessons()
	lessons := l.GetDefaultLessons()

	temp := make(map[string]int64)
	tempIdx := make([]string, 0)
	now := time.Now()
	for _, lesson := range lessons {
		if utils.DateIsBefore(lesson.StartsAt.Add(1*time.Hour), now) {
			timeStampString := lesson.StartsAt.Format("2006/01/02 15:04:05")
			layOut := "2006/01/02 15:04:05"
			timeStamp, _ := time.Parse(layOut, timeStampString)
			hr, _, _ := timeStamp.Clock()
			key := fmt.Sprintf("%d/%s/%d %d:00", lesson.StartsAt.Year(), lesson.StartsAt.Month(), lesson.StartsAt.Day(), hr)

			if _, ok := temp[key]; !ok {
				tempIdx = append(tempIdx, key)
			}
			temp[key]++
		}
	}

	res.Data = make([]XYPair, len(tempIdx))
	for i, key := range tempIdx {
		res.Data[i] = XYPair{X: key, Y: temp[key]}
	}

	c.JSON(http.StatusOK, res)
}

func instantSessionsMetricHandler(c *gin.Context) {
	auth.IsAdminMiddleware(c)
	res := sessionsMetricResponse{}
	l := services.GetLessons()
	lessons := l.GetInstantLessons()

	temp := make(map[string]int64)
	tempIdx := make([]string, 0)
	now := time.Now()
	for _, lesson := range lessons {
		if utils.DateIsBefore(lesson.StartsAt.Add(1*time.Hour), now) {
			if _, ok := temp[lesson.StartsAt.Format("2006/01/02")]; !ok {
				tempIdx = append(tempIdx, lesson.StartsAt.Format("2006/01/02"))
			}
			temp[lesson.StartsAt.Format("2006/01/02")]++
		}
	}
	res.Data = make([]XYPair, len(tempIdx))
	for i, key := range tempIdx {
		res.Data[i] = XYPair{X: key, Y: temp[key]}
	}

	c.JSON(http.StatusOK, res)
}

func instantSessionsHourlyMetricHandler(c *gin.Context) {
	auth.IsAdminMiddleware(c)
	res := sessionsMetricResponse{}
	l := services.GetLessons()
	lessons := l.GetInstantLessons()

	temp := make(map[string]int64)
	tempIdx := make([]string, 0)
	now := time.Now()
	for _, lesson := range lessons {
		if utils.DateIsBefore(lesson.StartsAt.Add(1*time.Hour), now) {
			timeStampString := lesson.StartsAt.Format("2006/01/02 15:04:05")
			layOut := "2006/01/02 15:04:05"
			timeStamp, _ := time.Parse(layOut, timeStampString)
			hr, _, _ := timeStamp.Clock()
			key := fmt.Sprintf("%d/%s/%d %d:00", lesson.StartsAt.Year(), lesson.StartsAt.Month(), lesson.StartsAt.Day(), hr)

			if _, ok := temp[key]; !ok {
				tempIdx = append(tempIdx, key)
			}
			temp[key]++
		}
	}
	res.Data = make([]XYPair, len(tempIdx))
	for i, key := range tempIdx {
		res.Data[i] = XYPair{X: key, Y: temp[key]}
	}

	c.JSON(http.StatusOK, res)
}

func Setup(g *gin.RouterGroup) {
	g.GET("/sessions", auth.IsAdminMiddleware, sessionsMetricHandler)
	g.GET("/sessions-hourly", auth.IsAdminMiddleware, sessionsHourlyMetricHandler)
	g.GET("/instant", auth.IsAdminMiddleware, instantSessionsMetricHandler)
	g.GET("/instant-hourly", auth.IsAdminMiddleware, instantSessionsHourlyMetricHandler)
}
