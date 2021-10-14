package cs

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/bieber/barcode"
	"github.com/karmdip-mi/go-fitz"
	"github.com/otiai10/gosseract/v2"
	"github.com/shopspring/decimal"
)

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
	err := scanMeta(b)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	doc := &Transaction{}
	return doc, doc.parse(b)
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

	err = td.parseTx(doc)
	if err != nil {
		return fmt.Errorf("parsing tx %w", err)
	}
	return nil
}

var (
	_accountNumberRe = regexp.MustCompile(`. Holder Account Number: (C\d{10})`)
	_cusipRe         = regexp.MustCompile(`CUSIP ([0-9]{3}[a-zA-Z0-9]{6})`)
	_transactionRe   = regexp.MustCompile(`(?P<date>\d{2} [a-zA-Z]{3} \d{4})(?P<description>\D*)(?P<transaction_amount>[0-9.,]+) (?P<deduction_amount>[0-9.,]+)(?P<deduction_type>\D*)(?P<net_amount>[0-9.,]+) (?P<price_per_share>[0-9.,]+) (?P<total_shares>[0-9.,]+)`)

	_transactionDateLayout = "2 Jan 2006"
)

func (td *Transaction) parseTx(doc *fitz.Document) error {
	txPageImg, err := doc.Image(0)
	if err != nil {
		return fmt.Errorf("reading transaction page %w", err)
	}

	boxes, err := boundingBoxesFromImage(txPageImg)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

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

	_, transactionHeaderIdx := findBoundingBox(boxes, point(84, 2043), point(312, 2080))
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
			td.Date, err = time.Parse(_transactionDateLayout, value)
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

	td.ID = strings.TrimSpace(boxes[len(boxes)-1].Word)

	return td.verify(doc)
}

func scanMeta(b []byte) error {
	s := bufio.NewScanner(bytes.NewReader(b))

	var (
		seenTrailer, seenMeta bool
		meta                  = map[string]string{}
		metaPrefix, trailer   = []byte("<</Creator"), []byte("trailer")
	)
	for s.Scan() {
		line := s.Bytes()
		if bytes.HasPrefix(line, trailer) {
			if seenTrailer {
				return errors.New("duplicate trailers")
			}
			seenTrailer = true
		}
		if bytes.HasPrefix(line, metaPrefix) {
			if seenMeta {
				return errors.New("duplicate meta")
			}
			seenMeta = true
			metaFields := strings.FieldsFunc(string(line[2:]), func(r rune) bool { return r == '/' })
			for _, metaKV := range metaFields {
				parts := strings.SplitN(metaKV, "(", 2)
				if len(parts) != 2 {
					continue // some leftover value
				}
				meta[parts[0]] = parts[1]
			}
		}
	}
	if !strings.HasPrefix(meta["Creator"], "Computershare Communication Services, GPD 3.00") {
		log.Printf("unusual invalid creator %v", meta["Creator"])
		return errors.New("invalid creator")
	}
	if !strings.HasPrefix(meta["Producer"], "PDFlib+PDI 7.0.4p1") {
		log.Printf("unusual invalid producer %v", meta["Producer"])
		return errors.New("invalid producer")
	}
	if strings.TrimSpace(meta["Subject"]) == "" {
		log.Printf("unusual empty subject %v", meta["Subject"])
		return errors.New("invalid subject")
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

func findBoundingBox(boxes []gosseract.BoundingBox, min, max image.Point) (gosseract.BoundingBox, int) {
	for idx, box := range boxes {
		if box.Box.Min.X == min.X && box.Box.Min.Y == min.Y &&
			box.Box.Max.X == max.X && box.Box.Max.Y == max.Y {
			return box, idx
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

func (td *Transaction) verify(doc *fitz.Document) error {
	if td.AccountID == "" {
		return errors.New("missing account ID")
	}
	for n := 0; n < doc.NumPage(); n++ {
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
	}
	// Basic math sanity check
	if !td.ClosePosition.Sub(td.OpenPosition).Equal(td.TotalShares) {
		return errors.New("basic position math is incorrect")
	}
	if !td.Amount.Sub(td.DeductionAmount).Equal(td.NetAmount) {
		return errors.New("basic amount math is incorrect")
	}
	if !td.TotalShares.Mul(td.PricePerShare).Round(2).Equal(td.NetAmount) {
		log.Printf("total=%v pps=%v %v!=%v", td.TotalShares, td.PricePerShare, td.TotalShares.Mul(td.PricePerShare).Round(2), td.NetAmount)
		return errors.New("basic price math is incorrect")
	}
	return nil
}

func accountID(boxes []gosseract.BoundingBox) (string, error) {
	acctBox, accountIdx := findBoundingBox(boxes, point(83, 1360), point(2352, 1419))
	if accountIdx < 0 {
		return "", errors.New("missing account number")
	}
	accountIDMatch := _accountNumberRe.FindStringSubmatch(acctBox.Word)
	if len(accountIDMatch) != 2 {
		log.Printf("unusual account number match len %d", len(accountIDMatch))
		return "", errors.New("incorrect account number matches")
	}
	accountID := accountIDMatch[1]
	if accountID == "" {
		return "", errors.New("empty account number")
	}
	return accountID, nil
}

func cusip(boxes []gosseract.BoundingBox) (string, error) {
	cusipBox, cusipIdx := findBoundingBoxPoint(boxes, point(1700, 1111))
	if cusipIdx < 0 {
		return "", errors.New("missing cusip")
	}
	cusipMatch := _cusipRe.FindStringSubmatch(cusipBox.Word)
	if len(cusipMatch) != 2 {
		log.Printf("unusual cusip match len %d", len(cusipMatch))
		return "", errors.New("incorrect cusip matches")
	}
	cusip := cusipMatch[1]
	if cusip == "" {
		return "", errors.New("empty cusip")
	}
	return cusip, nil
}

func positions(boxes []gosseract.BoundingBox) (open, close decimal.Decimal, err error) {
	_, sharePositionHeaderIdx := findBoundingBox(boxes, point(96, 1701), point(1742, 1777))
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

func point(x, y int) image.Point { return image.Point{x, y} }
