package indexer

import (
	"errors"

	"github.com/pemistahl/lingua-go"
)

const UnknownLanguage = "unknown"

var Languages = []lingua.Language{
	lingua.Arabic,     // ar
	lingua.Bulgarian,  // bg
	lingua.Catalan,    // ca
	lingua.Czech,      // cs
	lingua.Danish,     // da
	lingua.German,     // de
	lingua.Greek,      // el
	lingua.English,    // en
	lingua.Spanish,    // es
	lingua.Basque,     // eu
	lingua.Persian,    // fa
	lingua.Finnish,    // fi
	lingua.French,     // fr
	lingua.Irish,      // ga
	lingua.Hindi,      // hi
	lingua.Croatian,   // hr
	lingua.Hungarian,  // hu
	lingua.Armenian,   // hy
	lingua.Indonesian, // id
	lingua.Italian,    // it
	lingua.Dutch,      // nl
	lingua.Polish,     // pl
	lingua.Portuguese, // pt
	lingua.Romanian,   // ro
	lingua.Russian,    // ru
	lingua.Swedish,    // sv
	lingua.Turkish,    // tr
	// supported by bleve but not by lingua: no, gl, in
}

var langDetector = lingua.NewLanguageDetectorBuilder().FromLanguages(Languages...).Build()

func DetectLanguage(s string) (*lingua.Language, error) {
	if language, exists := langDetector.DetectLanguageOf(s); exists {
		return &language, nil
	}
	return nil, errors.New("unknown language")
}
