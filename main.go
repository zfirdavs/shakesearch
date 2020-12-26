package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"index/suffixarray"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

var fileName = flag.String("f", "completeworks.txt", "the name of file to read")

func main() {
	flag.Parse()

	searcher := Searcher{}
	err := searcher.Load(*fileName)
	if err != nil {
		log.Fatal(err)
	}

	fs := http.FileServer(http.Dir("./static"))
	mux := http.NewServeMux()
	mux.Handle("/", fs)

	mux.HandleFunc("/search", handleSearch(searcher))

	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Listening on port %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

type Searcher struct {
	CompleteWorks []byte
	SuffixArray   *suffixarray.Index
}

func handleSearch(searcher Searcher) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		buf := &bytes.Buffer{}
		query, ok := r.URL.Query()["q"]
		reqQuery := query[0]
		if !ok || len(reqQuery) < 1 {
			http.Error(w, "missing search query in URL params", http.StatusBadRequest)
			return
		}

		// found the lower case request query
		lowerQ := strings.ToLower(reqQuery)
		resultsLower := searcher.Search(lowerQ)

		// found the title case request query
		titleQ := strings.Title(reqQuery)
		resultsTitle := searcher.Search(titleQ)

		// found the upper case request query
		upperQ := strings.ToUpper(reqQuery)
		resultsUpper := searcher.Search(upperQ)

		totalLen := len(resultsLower) + len(resultsTitle) + len(resultsUpper)
		totalResults := make([]string, 0, totalLen)
		if len(resultsLower) != 0 {
			totalResults = append(totalResults, resultsLower...)
		}
		if len(resultsTitle) != 0 {
			totalResults = append(totalResults, resultsTitle...)
		}
		if len(resultsUpper) != 0 {
			totalResults = append(totalResults, resultsUpper...)
		}

		if err := json.NewEncoder(buf).Encode(totalResults); err != nil {
			http.Error(w, "encoding failure", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(buf.Bytes())
	}
}

func (s *Searcher) Load(fileName string) error {
	dat, err := ioutil.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("error read file: %w", err)
	}
	s.CompleteWorks = dat
	s.SuffixArray = suffixarray.New(dat)
	return nil
}

func (s *Searcher) Search(query string) []string {
	indexes := s.SuffixArray.Lookup([]byte(query), -1)
	results := []string{}
	for _, idx := range indexes {
		word := s.getWordsFromIndex(idx)
		results = append(results, s.TrimFunc(word))
	}
	return results
}

func (s *Searcher) getWordsFromIndex(index int) string {
	var wordStart, wordEnd int
	for i := index - 1; i >= 0; {
		r, size := utf8.DecodeRune(s.CompleteWorks[i:])
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			wordStart = i
			break
		}
		i -= size
	}

	for i := index + 1; i < len(s.CompleteWorks); {
		r, size := utf8.DecodeRune(s.CompleteWorks[i:])
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			wordEnd = i
			break
		}
		i += size
	}

	return string(s.CompleteWorks[wordStart:wordEnd])
}

func (s *Searcher) TrimFunc(inputStr string) string {
	return strings.TrimFunc(inputStr, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}
