package i18n

import (
	"reflect"
	"regexp"
	"testing"
)

var verbs = regexp.MustCompile(`%[^%]`)

func TestForPicksPackAndFallsBackToEnglish(t *testing.T) {
	for _, tc := range []struct {
		lang string
		want Pack
	}{
		{"Russian", russian},
		{" РУССКИЙ ", russian},
		{"Spanish", spanish},
		{"español", spanish},
		{"Klingon", english},
		{"", english},
	} {
		if got := For(tc.lang); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("For(%q) = %+v, want %+v", tc.lang, got, tc.want)
		}
	}
}

func TestPacksShareFormatVerbs(t *testing.T) {
	base := reflect.ValueOf(english)
	for _, pack := range []Pack{russian, spanish} {
		v := reflect.ValueOf(pack)
		for i := range base.NumField() {
			name := base.Type().Field(i).Name
			if n, m := len(verbs.FindAllString(base.Field(i).String(), -1)), len(verbs.FindAllString(v.Field(i).String(), -1)); n != m {
				t.Errorf("%s.%s has %d format verbs, english has %d", base.Type().Name(), name, m, n)
			}
		}
	}
}
