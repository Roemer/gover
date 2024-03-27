package gover

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// A simple regexp that matches one, two or three digits separated by a dot.
// d(.d)(.d)
var RegexpSimple *regexp.Regexp = regexp.MustCompile(`^(?P<d1>\d+)(?:\.(?P<d2>\d+))?(?:\.(?P<d3>\d+))?$`)

// A regex that matches the semantic versioning pattern.
// d.d.d(-s)(+s)
var RegexpSemver *regexp.Regexp = regexp.MustCompile(`^(?P<d1>\d+)\.(?P<d2>\d+)\.(?P<d3>\d+)(?:-(?P<s4>[^+]+))?(?:\+(?P<s5>.*))?$`)

// An empty version, can be used to find the max version of a list.
var EmptyVersion *Version = &Version{}

// Type that represents a version object.
type Version struct {
	Original string
	Segments []VersionSegment
}

// A segment of the version, can either be a number or a text.
type VersionSegment struct {
	Number       int
	Text         string
	IsText       bool
	IsNotDefined bool
}

// Converts the version to a readable string.
func (v *Version) String() string {
	strs := make([]string, len(v.Segments))
	for i, v := range v.Segments {
		strs[i] = v.String()
	}
	return strings.Join(strs, "|")
}

// Converts the version segment to a readable string.
func (v *VersionSegment) String() string {
	if v.IsNotDefined {
		return "-"
	}
	if v.IsText {
		return v.Text
	}
	return fmt.Sprintf("%d", v.Number)
}

func (v *Version) Major() int {
	if len(v.Segments) > 0 {
		return v.Segments[0].Number
	}
	return 0
}

func (v *Version) Minor() int {
	if len(v.Segments) > 1 {
		return v.Segments[1].Number
	}
	return 0
}

func (v *Version) Patch() int {
	if len(v.Segments) > 2 {
		return v.Segments[2].Number
	}
	return 0
}

func Compare(a *Version, b *Version) int {
	return a.CompareTo(b)
}

func (a *Version) CompareTo(b *Version) int {
	minSegments := min(len(a.Segments), len(b.Segments))
	for i := 0; i < minSegments; i++ {
		segmentA := a.Segments[i]
		segmentB := b.Segments[i]

		if segmentA.IsText || segmentB.IsText {
			if c := compareString(segmentA.Text, segmentB.Text); c != 0 {
				return c
			}
		} else {
			if c := compareInt(segmentA.Number, segmentB.Number); c != 0 {
				return c
			}
		}
	}
	// Favor the one with more segments
	return compareInt(len(a.Segments), len(b.Segments))
}

func Sort(versions []*Version) {
	slices.SortStableFunc(versions, Compare)
}

// Gets the maximum version which complies to a given version of a list of versions.
func FindMax(versions []*Version, reqVersion *Version, onlyWithoutStringValues bool) *Version {
	var max *Version = nil
	for _, v := range versions {
		isValid := true
		for i, s := range reqVersion.Segments {
			versionSegment := v.Segments[i]
			// Invalidate if no text is allowed
			if onlyWithoutStringValues && versionSegment.IsText {
				isValid = false
				break
			}
			// No requirement from the required version, so it is valid
			if s.IsNotDefined {
				continue
			}
			// Iinvalidate if the number does not match
			if s.Number != versionSegment.Number {
				isValid = false
				break
			}
		}
		if isValid {
			max = v
		}
	}
	return max
}

//////////
// Constructor methods
//////////

// Parses the given parts into a version.
func ParseSimple(parts ...interface{}) *Version {
	version := &Version{}
	for _, part := range parts {
		newSegment := VersionSegment{}
		switch v := part.(type) {
		case int:
			newSegment.Number = v
		default:
			newSegment.Text = fmt.Sprintf("%v", v)
			newSegment.IsText = true
		}
		version.Segments = append(version.Segments, newSegment)
	}
	return version
}

func MustParseVersionFromRegex(versionString string, versionRegexp *regexp.Regexp) *Version {
	return must(ParseVersionFromRegex(versionString, versionRegexp))
}

// Parses the given version string with the regexp into the version object.
func ParseVersionFromRegex(versionString string, versionRegexp *regexp.Regexp) (*Version, error) {
	matchMap := findNamedMatches(versionRegexp, versionString, true)
	if matchMap == nil {
		return nil, fmt.Errorf("failed parsing the version: %s", versionString)
	}

	// Build a map with index and the segments
	insertMap := map[int]VersionSegment{}
	for k, v := range matchMap {
		// Get the index of the current segment being processed
		index, err := strconv.Atoi(k[1:])
		if err != nil {
			return nil, fmt.Errorf("invalid format for group name: %s", k)
		}
		// Build the new segment
		newSegment := VersionSegment{}
		if v == "" {
			// Undefined
			newSegment.IsNotDefined = true
		} else if k[0] == 's' {
			// String
			newSegment.Text = v
			newSegment.IsText = true
		} else {
			// Number
			num, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid value for number group %s: %s", k, v)
			}
			newSegment.Number = num
		}
		// Insert the new segment to the map
		insertMap[index] = newSegment
	}

	// Add the segments in the correct order
	parsedVersion := &Version{Original: versionString}
	index := 1
	for {
		if value, ok := insertMap[index]; !ok {
			break
		} else {
			parsedVersion.Segments = append(parsedVersion.Segments, value)
		}
		index++
	}
	// Return it
	return parsedVersion, nil
}

//////////
// Internal methods
//////////

func compareInt(a int, b int) int {
	if a > b {
		return 1
	}
	if b > a {
		return -1
	}
	return 0
}

// Compares two strings ignoring case. An empty string is peferred to a defined string.
func compareString(a string, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

func findNamedMatches(regex *regexp.Regexp, str string, includeNotMatchedOptional bool) map[string]string {
	match := regex.FindStringSubmatchIndex(str)
	if match == nil {
		// No matches
		return nil
	}
	subexpNames := regex.SubexpNames()
	results := map[string]string{}
	// Loop thru the subexp names (skipping the first empty one)
	for i, name := range (subexpNames)[1:] {
		startIndex := match[i*2+2]
		endIndex := match[i*2+3]
		if startIndex == -1 || endIndex == -1 {
			// No match found
			if includeNotMatchedOptional {
				// Add anyways
				results[name] = ""
			}
			continue
		}
		// Assign the correct value
		results[name] = str[startIndex:endIndex]
	}
	return results
}

func must[T any](obj T, err error) T {
	if err != nil {
		panic(err)
	}
	return obj
}
