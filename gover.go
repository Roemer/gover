package gover

import (
	"cmp"
	"errors"
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

var (
	ErrNoMatch = errors.New("failed matching")
)

// Type that represents a version object.
type Version struct {
	Raw      string
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
			if c := cmp.Compare(segmentA.Number, segmentB.Number); c != 0 {
				return c
			}
		}
	}
	// Favor the one with more segments
	return cmp.Compare(len(a.Segments), len(b.Segments))
}

func (a *Version) GreaterThan(b *Version) bool {
	return a.CompareTo(b) == 1
}

func (a *Version) LessThan(b *Version) bool {
	return a.CompareTo(b) == -1
}

func (a *Version) Equals(b *Version) bool {
	return a.CompareTo(b) == 0
}

func Sort(versions []*Version) {
	slices.SortStableFunc(versions, Compare)
}

func FindMaxGeneric[T any](versions []T, getFunc func(x T) *Version, referenceVersion *Version, onlyWithoutStringValues bool) T {
	var max *Version = nil
	var maxObject T
	for _, v := range versions {
		version := getFunc(v)
		isValid := true
		// Loop thru the segments of a possible candidate
		for i, versionSegment := range version.Segments {
			// Invalidate if no text is allowed
			if onlyWithoutStringValues && versionSegment.IsText {
				isValid = false
				break
			}
			// Check if the segment of the reference version matches
			if len(referenceVersion.Segments) > i {
				referenceSegment := referenceVersion.Segments[i]
				// No requirement from the reference version, so it is valid
				if referenceSegment.IsNotDefined {
					continue
				}
				// Invalidate if the number does not match
				if referenceSegment.Number != versionSegment.Number {
					isValid = false
					break
				}
			}
		}
		if isValid {
			if max == nil || version.GreaterThan(max) {
				max = version
				maxObject = v
			}
		}
	}
	return maxObject
}

// Gets the maximum version which complies to a given version of a list of versions.
func FindMax(versions []*Version, referenceVersion *Version, onlyWithoutStringValues bool) *Version {
	return FindMaxGeneric(versions, func(x *Version) *Version { return x }, referenceVersion, onlyWithoutStringValues)
}

//////////
// Constructor methods
//////////

// Parses the given parts into a version.
func ParseSimple(parts ...interface{}) *Version {
	version := &Version{}
	for _, part := range parts {
		segmentsToAdd := []VersionSegment{}
		switch v := part.(type) {
		case int:
			segmentsToAdd = append(segmentsToAdd, VersionSegment{
				Number: v,
			})
		case string:
			segmentsToAdd = append(segmentsToAdd, buildSegmentFromString(v))
		case []int:
			for _, x := range v {
				segmentsToAdd = append(segmentsToAdd, VersionSegment{
					Number: x,
				})
			}
		case []string:
			for _, x := range v {
				segmentsToAdd = append(segmentsToAdd, buildSegmentFromString(x))
			}
		default:
			// Conver the value to string
			str := fmt.Sprintf("%v", v)
			segmentsToAdd = append(segmentsToAdd, buildSegmentFromString(str))
		}
		// Add all the new segments
		version.Segments = append(version.Segments, segmentsToAdd...)
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
		return nil, fmt.Errorf("failed parsing the version %s: %w", versionString, ErrNoMatch)
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
		} else if k[0] == 'd' {
			// Number
			num, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid value for number group %s: %s", k, v)
			}
			newSegment.Number = num
		} else {
			// Anything else, dynamically create text or number segment
			newSegment = buildSegmentFromString(v)
		}
		// Insert the new segment to the map
		insertMap[index] = newSegment
	}

	// Add the segments in the correct order
	parsedVersion := &Version{Raw: versionString}
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
		if name == "" {
			// No name, so automatically give it a name
			name = fmt.Sprintf("p%d", (i + 1))
		}
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

// Converts a string to a segment
func buildSegmentFromString(value string) VersionSegment {
	// First try to convert to integer
	if n, err := strconv.Atoi(value); err == nil {
		return VersionSegment{
			Number: n,
		}
	}
	// Failed, so just create a text segment
	return VersionSegment{
		Text:   value,
		IsText: true,
	}
}

func must[T any](obj T, err error) T {
	if err != nil {
		panic(err)
	}
	return obj
}
