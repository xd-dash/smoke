// Package versioning defines portable CalVer and immutable reference contracts.
// It does not authorize releases or depend on any artifact backend.
package versioning

import (
 "errors"
 "fmt"
 "regexp"
 "strconv"
 "time"
)

var calverRE = regexp.MustCompile(`^([0-9]{4})\.(0[1-9]|1[0-2])\.(0[1-9]|[12][0-9]|3[01])(?:\.(0|[1-9][0-9]*))?$`)

var ErrInvalidVersion = errors.New("invalid CalVer")

// Version is a UTC calendar release version YYYY.0M.0D[.MICRO].
type Version struct {
 Date time.Time
 Micro uint64
 HasMicro bool
}

// Parse validates the calendar date and canonical numeric spelling.
func Parse(s string) (Version, error) {
 m := calverRE.FindStringSubmatch(s)
 if m == nil { return Version{}, fmt.Errorf("%w: %q", ErrInvalidVersion, s) }
 year, _ := strconv.Atoi(m[1])
 month, _ := strconv.Atoi(m[2])
 day, _ := strconv.Atoi(m[3])
 d := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
 if year == 0 || d.Year()!=year || int(d.Month())!=month || d.Day()!=day {
  return Version{}, fmt.Errorf("%w: invalid date %q", ErrInvalidVersion, s)
 }
 v := Version{Date:d}
 if m[4]!="" {
  n,err:=strconv.ParseUint(m[4],10,64)
  if err!=nil { return Version{},fmt.Errorf("%w: micro: %v",ErrInvalidVersion,err) }
  v.Micro=n; v.HasMicro=true
 }
 return v,nil
}

func (v Version) String() string {
 if v.Date.IsZero() { return "" }
 base:=v.Date.UTC().Format("2006.01.02")
 if v.HasMicro { return base+"."+strconv.FormatUint(v.Micro,10) }
 return base
}

// Compare orders versions chronologically, then by micro revision.
// The unqualified date sorts before explicit .0.
func (v Version) Compare(other Version) int {
 if v.Date.Before(other.Date) { return -1 }
 if v.Date.After(other.Date) { return 1 }
 if !v.HasMicro && other.HasMicro { return -1 }
 if v.HasMicro && !other.HasMicro { return 1 }
 if v.Micro<other.Micro { return -1 }
 if v.Micro>other.Micro { return 1 }
 return 0
}
