package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func main() {
	ctx := context.Background()
	b, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	// Prints the names and majors of students in a sample spreadsheet:
	// https://docs.google.com/spreadsheets/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edit
	spreadsheetId := "1h_kGGAw5Q4lcox9zLwWqYD662DUlWcuU86GSgbF8g1Q"
	readRange := "AllLoans!A2:G"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	var LoanInfos []*LoanInfo
	if len(resp.Values) == 0 {
		fmt.Println("No data found.")
		return
	}

	for _, row := range resp.Values {
		loanProvider := row[0].(string)
		loanStartDate, err := time.Parse("1/2/2006", row[1].(string))
		if err != nil {
			fmt.Printf("error whild parsing start date from provider %s: %s\n", loanProvider, err.Error())
		}
		loanAmount, err := strconv.ParseFloat(strings.Replace(strings.TrimPrefix(row[2].(string), "$"), ",", "", -1), 64)
		if err != nil {
			fmt.Printf("error whild parsing loan amount from provider %s: %s\n", loanProvider, err.Error())
		}
		interestRate, err := strconv.ParseFloat(strings.TrimSuffix(row[3].(string), "%"), 32)
		if err != nil {
			fmt.Printf("error whild parsing interest rate from provider %s: %s\n", loanProvider, err.Error())
		}
		loanTerm, err := strconv.Atoi(strings.TrimPrefix(row[4].(string), "$"))
		if err != nil {
			fmt.Printf("error whild parsing loan term from provider %s: %s\n", loanProvider, err.Error())
		}
		monthlyPayment, err := strconv.ParseFloat(strings.TrimPrefix(row[5].(string), "$"), 32)
		if err != nil {
			fmt.Printf("error whild parsing monthly payment from provider %s: %s\n", loanProvider, err.Error())
		}
		var additionalMonthlyPayment float64
		if row[6].(string) != "" {
			additionalMonthlyPayment, err = strconv.ParseFloat(strings.TrimPrefix(row[6].(string), "$"), 32)
			if err != nil {
				fmt.Printf("error whild parsing additional monthly payment from provider %s: %s\n", loanProvider, err.Error())
			}
		}
		// Handle column data. Column A is 0th index
		LoanInfos = append(LoanInfos, &LoanInfo{
			Name:              loanProvider,
			StartDate:         loanStartDate,
			Amount:            float32(loanAmount),
			InterestRate:      float32(interestRate) / 100,
			Term:              loanTerm,
			Payment:           float32(monthlyPayment),
			AdditionalPayment: float32(additionalMonthlyPayment),
		})

		// go till empty row
		if row[0] == "" {
			break
		}
	}

	earliestStart := time.Now()
	for _, v := range LoanInfos {
		if earliestStart.After(v.StartDate) {
			earliestStart = v.StartDate
		}
	}

	var rollOver float32

	dateTracker := earliestStart
	resultData := map[time.Time]map[int]float32{}
	remainder := float32(0.0)
	for {
		resultData[dateTracker] = map[int]float32{}
		anyPositiveRemaining := false
		for _, v := range LoanInfos {
			if v.StartDate.After(dateTracker) || v.Amount < 0.01 {
				continue
			}

			payment := v.Payment + v.AdditionalPayment
			if payment > v.Amount {
				remainder += payment - v.Amount
				// if amount hits zero, the monthly payment of this rolls into the active account with the highest interest
				rollOver += payment
				v.Amount = 0
			} else {
				v.Amount -= payment
			}

			if v.Amount > 0 {
				anyPositiveRemaining = true
			}
			if remainder > 0 {
				fmt.Printf("%f leftover after paying %s\n", remainder, v.Name)
			}
		}

		// try to pay extra to the highest interest
		if remainder > 0 || rollOver > 0 {
			var highestInterestLoan int
			highestInterest := float32(0.0)
			for i, v := range LoanInfos {
				if v.StartDate.After(dateTracker) || v.Amount < 0.01 {
					continue
				}

				if v.InterestRate > highestInterest {
					highestInterest = v.InterestRate
					highestInterestLoan = i
				}
			}

			if highestInterest > 0.0 {
				v := LoanInfos[highestInterestLoan]
				payment := remainder + rollOver
				remainder = 0
				if payment > v.Amount {
					// let this new remainder roll over for simplicity
					remainder += payment - v.Amount
					v.Amount = 0
				} else {
					v.Amount -= payment
				}
			}
		}

		// track amount after all payments
		for i, v := range LoanInfos {
			resultData[dateTracker][i] = v.Amount
		}

		if !anyPositiveRemaining {
			break
		}

		for _, v := range LoanInfos {
			v.Amount += v.Amount * (v.InterestRate / 12)
		}
		dateTracker = dateTracker.AddDate(0, 1, 0)
	}
	dateTracker = earliestStart

	headers := "date, "
	// loop loan info to maintain consistent order
	for _, v := range LoanInfos {
		headers += v.Name + ", "
	}
	fmt.Println()
	fmt.Println(headers)
	for {
		if _, ok := resultData[dateTracker]; !ok {
			break
		}

		row := fmt.Sprint(dateTracker.Format("1/2/2006"), ", ")
		// loop loan info to maintain consistent order
		for i := 0; i < len(LoanInfos); i++ {
			row += fmt.Sprintf("%f, ", resultData[dateTracker][i])
		}
		fmt.Println(row)
		dateTracker = dateTracker.AddDate(0, 1, 0)
	}
}

type LoanInfo struct {
	Name              string
	StartDate         time.Time
	Amount            float32
	InterestRate      float32
	Term              int
	Payment           float32
	AdditionalPayment float32
}
