package tfl

import "time"

type gmtConverter struct {
	loc *time.Location
}

func newGMTConverter() *gmtConverter {
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		panic(err)
	}
	return &gmtConverter{
		loc: loc,
	}
}

func (g *gmtConverter) convert(input time.Time) time.Time {
	return input.In(g.loc)
}

var gmtc = newGMTConverter()
