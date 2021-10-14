package cs

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bieber/barcode"
	"github.com/connyay/drclub/store"
	"github.com/karmdip-mi/go-fitz"
	"github.com/otiai10/gosseract/v2"
	"github.com/shopspring/decimal"
)

var (
	_accountNumberRe = regexp.MustCompile(`. Holder Account Number: (C0{3}\d{7})`) // note: allows for millions of account numbers. Will break if tens of millions :)
	_cusipRe         = regexp.MustCompile(`CUSIP ([0-9]{3}[a-zA-Z0-9]{6})`)
	// I'm really sorry for this regex. https://regex101.com/r/oyS0Kw/1
	_transactionRe = regexp.MustCompile(`(?P<date>\d{2} [a-zA-Z]{3} \d{4})(?P<description>\D*)(?P<transaction_amount>[0-9.,]+) (?P<deduction_amount>[0-9.,]+)(?P<deduction_type>\D*)(?P<net_amount>[0-9.,]+) (?P<price_per_share>[0-9.,]+) (?P<total_shares>[0-9.,]+)`)

	// Salts to prevent brute forcing to reverse account numbers.
	_txIDSalt      = os.Getenv("TX_ID_SALT")
	_accountIDSalt = os.Getenv("ACCOUNT_ID_SALT")
)

// Transaction is parsed from a transaction statement.
type Transaction struct {
	ID              string
	Date            time.Time
	AccountID       string
	CUSIP           string
	Description     string
	DeductionType   string
	OpenPosition    decimal.Decimal
	ClosePosition   decimal.Decimal
	Amount          decimal.Decimal
	DeductionAmount decimal.Decimal
	NetAmount       decimal.Decimal
	PricePerShare   decimal.Decimal
	TotalShares     decimal.Decimal
}

func Parse(b []byte) (*Transaction, error) {
	err := validateMeta(b)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	doc := &Transaction{}
	err = doc.parse(b)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	return doc, nil
}

func (td *Transaction) ToStore(requester string) store.Transaction {
	return store.Transaction{
		IDHash:          fmt.Sprintf("%x", sha256.Sum256([]byte(_txIDSalt+td.ID))),
		AccountIDHash:   fmt.Sprintf("%x", sha256.Sum256([]byte(_accountIDSalt+td.AccountID))),
		Date:            td.Date,
		RequesterHash:   requester,
		CUSIP:           td.CUSIP,
		Description:     td.Description,
		Amount:          td.Amount,
		DeductionAmount: td.DeductionAmount,
		NetAmount:       td.NetAmount,
		PricePerShare:   td.PricePerShare,
		TotalShares:     td.TotalShares,
	}
}

func (td *Transaction) parse(b []byte) error {
	doc, err := fitz.NewFromMemory(b)
	if err != nil {
		return fmt.Errorf("parsing document %w", err)
	}
	defer doc.Close()

	if doc.NumPage() != 2 {
		log.Printf("unexpected number of pages expected 2 got %d", doc.NumPage())
		return errors.New("unexpected number of pages")
	}

	txPageImg, err := doc.Image(0)
	if err != nil {
		return fmt.Errorf("reading transaction page %w", err)
	}

	boxes, err := boundingBoxesFromImage(txPageImg)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// FIXME - validate the format of this. Only have a small sample size to
	// test with, and ocr has been flaky getting this exact.
	td.ID = strings.TrimSpace(boxes[len(boxes)-1].Word)

	td.AccountID, err = accountID(boxes)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	td.CUSIP, err = cusip(boxes)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	td.OpenPosition, td.ClosePosition, err = positions(boxes)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	err = td.transaction(boxes)
	if err != nil {
		return fmt.Errorf("parsing tx %w", err)
	}

	err = td.verify(doc)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func accountID(boxes []gosseract.BoundingBox) (string, error) {
	acctBB, accountIdx := findBoundingBox(boxes, image.Rect(83, 1360, 2352, 1419))
	if accountIdx < 0 {
		return "", errors.New("missing account number")
	}
	match := _accountNumberRe.FindStringSubmatch(acctBB.Word)
	if len(match) != 2 {
		log.Printf("unusual account number match len %d", len(match))
		return "", errors.New("incorrect account number matches")
	}
	accountID := match[1]
	if accountID == "" {
		return "", errors.New("empty account number")
	}
	return accountID, nil
}

func cusip(boxes []gosseract.BoundingBox) (string, error) {
	cusipBB, cusipIdx := findBoundingBoxPoint(boxes, image.Pt(1700, 1111))
	if cusipIdx < 0 {
		return "", errors.New("missing cusip")
	}
	match := _cusipRe.FindStringSubmatch(cusipBB.Word)
	if len(match) != 2 {
		log.Printf("unusual cusip match len %d", len(match))
		return "", errors.New("incorrect cusip matches")
	}
	cusip := match[1]
	if cusip == "" {
		return "", errors.New("empty cusip")
	}
	return cusip, nil
}

func positions(boxes []gosseract.BoundingBox) (open, close decimal.Decimal, err error) {
	_, sharePositionHeaderIdx := findBoundingBox(boxes, image.Rect(96, 1701, 1742, 1777))
	if sharePositionHeaderIdx < 0 {
		return open, close, errors.New("missing share position header")
	}
	if sharePositionIdx := sharePositionHeaderIdx + 1; sharePositionIdx <= len(boxes) {
		sharePositionBox := boxes[sharePositionIdx]
		sharePositions := strings.Split(strings.TrimSpace(sharePositionBox.Word), " ")
		if len(sharePositions) != 2 {
			log.Printf("unexpected position matches expected 2 got %d", len(sharePositions))
			return open, close, errors.New("incorrect share positions")
		}
		open, err = decimal.NewFromString(sharePositions[0])
		if err != nil {
			return open, close, fmt.Errorf("%w", err)
		}
		close, err = decimal.NewFromString(sharePositions[1])
		if err != nil {
			return open, close, fmt.Errorf("%w", err)
		}
	}
	return open, close, nil
}

func (td *Transaction) transaction(boxes []gosseract.BoundingBox) (err error) {
	_, transactionHeaderIdx := findBoundingBox(boxes, image.Rect(84, 2043, 312, 2080))
	if transactionHeaderIdx < 0 {
		return errors.New("missing transaction header")
	}

	transactionBox := boxes[transactionHeaderIdx+3]
	match := _transactionRe.FindStringSubmatch(transactionBox.Word)

	decimalFromStr := func(s string) (decimal.Decimal, error) {
		return decimal.NewFromString(strings.ReplaceAll(s, ",", "")) // 10,000.00 -> 10000.00
	}
	for i, name := range _transactionRe.SubexpNames() {
		value := strings.TrimSpace(match[i])
		switch name {
		case "date":
			layout := "2 Jan 2006"
			td.Date, err = time.Parse(layout, value)
		case "description":
			td.Description = value
		case "transaction_amount":
			td.Amount, err = decimalFromStr(value)
		case "deduction_amount":
			td.DeductionAmount, err = decimalFromStr(value)
		case "deduction_type":
			td.DeductionType = value
		case "net_amount":
			td.NetAmount, err = decimalFromStr(value)
		case "price_per_share":
			td.PricePerShare, err = decimalFromStr(value)
		case "total_shares":
			td.TotalShares, err = decimalFromStr(value)
		}
		if err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

func (td *Transaction) verify(doc *fitz.Document) error {
	if td.AccountID == "" {
		return errors.New("missing account ID")
	}
	// for n := 0; n < doc.NumPage(); n++ {
	// only validating the first page for now to reduce load. If abuse
	// becomes rampant will likely have to scan all pages
	n := 0
	img, err := doc.Image(n)
	if err != nil {
		return fmt.Errorf("reading image %w", err)
	}
	src := barcode.NewImage(img)
	scanner := barcode.NewScanner().
		SetEnabledAll(true)
	symbols, err := scanner.ScanImage(src)
	if err != nil {
		return fmt.Errorf("scanning image on page %d %w", n, err)
	}
	if len(symbols) != 1 {
		log.Printf("unusual barcode symbol len %d", len(symbols))
		return errors.New("unexpected number of symbols")
	}

	accountBarcode := symbols[0]

	if accountBarcode.Type != barcode.Code39 {
		log.Printf("unusual barcode type %v", accountBarcode.Type)
		return errors.New("unexpected symbol type")
	}
	// Quality is a bit of a guess. It is an unscaled value, so it could be
	// anything.
	if accountBarcode.Quality < 100 {
		log.Printf("unusual barcode quality %v", accountBarcode.Quality)
		return errors.New("unexpected symbol quality")
	}

	if accountBarcode.Data != td.AccountID {
		log.Printf("accountID not verified expected %q got %q", td.AccountID, accountBarcode.Data)
		return errors.New("accountID not verified")
	}
	// }
	// Basic math sanity check
	switch {
	case !td.ClosePosition.Sub(td.OpenPosition).Equal(td.TotalShares):
		return errors.New("basic position math is incorrect")
	case !td.Amount.Sub(td.DeductionAmount).Equal(td.NetAmount):
		return errors.New("basic amount math is incorrect")
	case !td.TotalShares.Mul(td.PricePerShare).Round(2).Equal(td.NetAmount):
		log.Printf("total=%v pps=%v %v!=%v", td.TotalShares, td.PricePerShare, td.TotalShares.Mul(td.PricePerShare).Round(2), td.NetAmount)
		return errors.New("basic price math is incorrect")
	}

	return nil
}

func boundingBoxesFromImage(src image.Image) ([]gosseract.BoundingBox, error) {
	var pagePng bytes.Buffer
	err := png.Encode(&pagePng, src)
	if err != nil {
		return nil, fmt.Errorf("encoding page png %w", err)
	}
	client := gosseract.NewClient()
	defer client.Close()

	err = client.SetImageFromBytes(pagePng.Bytes())
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	bbs, err := client.GetBoundingBoxes(gosseract.RIL_TEXTLINE)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	// For desparate debugging of bounding box positions.
	// jsonBytes, _ := json.MarshalIndent(bbs, "", "  ")
	// os.WriteFile(fmt.Sprintf("bb_%d.json", time.Now().UnixNano()), jsonBytes, 0644)
	return bbs, nil
}

func findBoundingBox(boxes []gosseract.BoundingBox, box image.Rectangle) (gosseract.BoundingBox, int) {
	for idx, b := range boxes {
		if b.Box == box {
			return b, idx
		}
	}
	return gosseract.BoundingBox{}, -1
}

func findBoundingBoxPoint(boxes []gosseract.BoundingBox, point image.Point) (gosseract.BoundingBox, int) {
	for idx, box := range boxes {
		if box.Box.Min.X <= point.X && box.Box.Min.Y <= point.Y &&
			box.Box.Max.X >= point.X && box.Box.Max.Y >= point.Y {
			return box, idx
		}
	}
	return gosseract.BoundingBox{}, -1
}
