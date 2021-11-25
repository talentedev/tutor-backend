package pdf

import (
	"fmt"
	"io"
	"time"

	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/store"

	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"
)

var now = time.Now()

const (
	cellWidthRef      = 30
	cellWidthStudents = 0
	cellWidthStatus   = 30
	cellWidthAmount   = 20
	cellWidthDate     = 30
	cellHeight        = 10
)

type transactions struct{}

var instanceTransactions = &transactions{}

func Transactions() *transactions {
	return instanceTransactions
}

func (t *transactions) Serve(c *gin.Context, user *store.UserMgo, items []*store.TransactionDto, from, to time.Time) (err error) {
	return t.Write(c.Writer, user, items, from, to)
}

func (t *transactions) Write(w io.Writer, user *store.UserMgo, items []*store.TransactionDto, from, to time.Time) (err error) {
	pageNum := 1

	pdf := gofpdf.New("P", "mm", "A4", "")

	width, height := pdf.GetPageSize()

	page := func() {
		pdf.AddPage()

		logoPath := "./resources/logo.png"
		if core.IsDebugging() {
			logoPath = "./src/api/cmd/pdf/logo.png"
		}
		pdf.Image(logoPath, 10, 10, 30, 0, false, "", 0, "https://learnt.io")

		pdf.SetFont("Courier", "", 11)
		pdf.Text(46, 17, "https://learnt.io")

		pdf.SetFont("Arial", "B", 16)
		pdf.Line(10, 25, width-10, 25)
		pdf.Line(10, height-15, width-10, height-15)

		rightMsg := fmt.Sprintf("Transactions of %s", from.Format("January 2006"))
		pdf.Text(width-pdf.GetStringWidth(rightMsg)-10, 20, rightMsg)

		// Write footer generated time
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(205, 205, 205)
		footerMsg := fmt.Sprintf("Generated %s", now.Format("Jan 2 2006 15:04:05"))
		pdf.Text(10, height-7, footerMsg)

		pdf.Text(width-20, height-7, fmt.Sprintf("Page %d", pageNum))

		pdf.SetTextColor(0, 0, 0)
		pdf.SetXY(10, 30)

		pageNum++
	}

	page()

	var row = 1

	for _, item := range items {
		var description string
		if item.Lesson == nil {
			description = item.Details
		} else {
			description = item.Lesson.StudentsNames(false)
		}

		var amount string
		if item.Amount > 0 {
			amount = fmt.Sprintf("$%.2f", item.Amount)
		} else {
			amount = fmt.Sprintf("-$%.2f", -item.Amount)
		}

		pdf.Cell(cellWidthRef, cellHeight, item.Reference)
		pdf.Cell(width-cellWidthStatus-cellWidthAmount-cellWidthDate-50, cellHeight, description)
		pdf.Cell(cellWidthStatus, cellHeight, item.Status)
		pdf.Cell(cellWidthAmount, cellHeight, amount)
		pdf.CellFormat(cellWidthDate, cellHeight, item.Time.Format("02 Jan 2006"), "", 0, "R", false, 0, "")

		Y := row*cellHeight + 30

		pdf.SetY(float64(Y))

		row++

		if pdf.GetY() > height-30 {
			row = 1
			page()
		}
	}

	return pdf.Output(w)
}
