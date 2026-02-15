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
	// The raw string of the version.
	Raw string
	// The different segements of the version.
	Segments []VersionSegment
	// A field for custom data for the version object.
	CustomData any
}

// A segment of the version, can either be a number or a text.
type VersionSegment struct {
	Number       int
	Text         string
	IsText       bool
	IsNotDefined bool
}

// Returns a boolean if the segment has a defined value or not.
func (v *VersionSegment) IsDefined() bool {
	return !v.IsNotDefined
}

// Converts the version to a readable string.
func (v *Version) String() string {
	strs := make([]string, len(v.Segments))
	for i, v := range v.Segments {
		strs[i] = v.String()
	}
	return strings.Join(strs, "|")
}

// Returns the total count of the segments, either with undefined or without.
func (v *Version) SegmentCount(onlyDefined bool) int {
	count := 0
	for _, segment := range v.Segments {
		if onlyDefined && segment.IsNotDefined {
			continue
		}
		count++
	}
	return count
}

// Returns the count of defined segments until an undefined one (or no more).
func (v *Version) DefinedSegmentCount() int {
	count := 0
	for _, segment := range v.Segments {
		if segment.IsNotDefined {
			break
		}
		count++
	}
	return count
}

// CoreVersion Converts the version to a core SemVer string in the form major.minor.patch.
func (v *Version) CoreVersion() string {
	strs := []string{}
	for i := 0; i < 3; i++ {
		if len(v.Segments) > i && !v.Segments[i].IsText && !v.Segments[i].IsNotDefined {
			strs = append(strs, v.Segments[i].String())
		} else {
			break
		}
	}

	for i := len(strs); i < 3; i++ {
		strs = append(strs, "0")
	}

	return strings.Join(strs, ".")
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

// MatchesConstraints checks if the given version satisfies a version constraint expression.
// Constraints can be combined using logical operators:
// - Space-separated constraints are combined with AND logic (all must match).
// - Use "||" to separate constraint sets with OR logic (at least one set must match).
// - Use "&&" as an alternative to space for AND logic.
//
// Supported operators:
// - Comparison: <, <=, >, >=, ==, !=, = (alias for ==)
// - Range: "start - end" (equivalent to ">=start <=end")
// - Caret (^): Compatible versions, e.g., ^1.2.3 allows 1.x.x but not 2.x.x
// - Tilde (~): Patch-level changes, e.g., ~1.2.3 allows 1.2.x
// - Wildcard (*): Partial matching, e.g., "1.*" matches 1.x.x, "1.2.*" matches 1.2.x
// - Regex: =~ (matches), !~ (does not match) against the version string
//
// Examples:
// - "<1.0.0": version < 1.0.0
// - ">=2.0.0": version >= 2.0.0
// - ">=1.0.0 <2.0.0": version in [1.0.0, 2.0.0)
// - "1.0.0 - 2.0.0": same as above
// - "^1.2.3": compatible with 1.x.x
// - "~1.2.3": patch-level changes within 1.2.x
// - "1.*": any 1.x.x version
// - "=~^1\\..*": regex match for versions starting with 1.
// - "==1.0.0 || ==2.0.0": exactly 1.0.0 or 2.0.0
func (v *Version) MatchesConstraints(constraint string) (bool, error) {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return true, nil // No constraint means any version is acceptable
	}
	// Split by "||" for OR logic
	constraintSets := strings.Split(constraint, "||")
	for _, set := range constraintSets {
		set = strings.TrimSpace(set)
		if set == "" {
			continue
		}
		if matches, err := v.matchesConstraintSet(set); err != nil {
			return false, err
		} else if matches {
			return true, nil
		}
	}
	return false, nil
}

func (v *Version) matchesConstraintSet(set string) (bool, error) {
	// Handle range with hyphen
	if strings.Contains(set, " - ") && !strings.ContainsAny(set, "<>=") {
		parts := strings.SplitN(set, " - ", 2)
		set = fmt.Sprintf(">=%s <=%s", strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	}

	// Replace explicit && with space for AND logic
	set = strings.ReplaceAll(set, "&&", " ")
	// Split by space for AND logic
	constraints := strings.Fields(set)
	for _, constraint := range constraints {
		if matches, err := v.matchesSingleConstraint(constraint); err != nil {
			return false, err
		} else if !matches {
			return false, nil
		}
	}
	return true, nil
}

func (v *Version) matchesSingleConstraint(constraint string) (bool, error) {
	// Handle Regex
	if strings.HasPrefix(constraint, "=~") || strings.HasPrefix(constraint, "!~") {
		operator := constraint[:2]
		constraintVersionString := strings.TrimSpace(constraint[2:])
		re, err := regexp.Compile(constraintVersionString)
		if err != nil {
			return false, fmt.Errorf("invalid regex in constraint: %w", err)
		}
		raw := v.Raw
		if raw == "" {
			raw = v.CoreVersion()
		}
		matched := re.MatchString(raw)
		if operator == "!~" {
			return !matched, nil
		}
		return matched, nil
	}

	// Caret
	if strings.HasPrefix(constraint, "^") {
		constraintVersionString := strings.TrimSpace(constraint[1:])
		constraintVersion, err := ParseVersionFromRegex(constraintVersionString, RegexpSimple)
		if err != nil {
			return false, err
		}
		if constraintVersion.Major() > 0 {
			// For major version > 0, only the major version is significant
			return v.GreaterThanOrEqual(constraintVersion) && v.LessThan(ParseSimple(constraintVersion.Major()+1, 0, 0)), nil
		}
		if constraintVersion.Minor() > 0 {
			// For major version 0 and minor version > 0, only the major and minor versions are significant
			return v.GreaterThanOrEqual(constraintVersion) && v.LessThan(ParseSimple(0, constraintVersion.Minor()+1, 0)), nil
		} else {
			// For major version 0 and minor version 0, only the patch version is significant
			return v.GreaterThanOrEqual(constraintVersion) && v.LessThan(ParseSimple(0, 0, constraintVersion.Patch()+1)), nil
		}
	}

	// Tilde
	if strings.HasPrefix(constraint, "~") {
		constraintVersionString := strings.TrimSpace(constraint[1:])
		constraintVersion, err := ParseVersionFromRegex(constraintVersionString, RegexpSimple)
		if err != nil {
			return false, err
		}
		// The patch version is not significant, but the major and minor versions are
		return v.GreaterThanOrEqual(constraintVersion) && v.LessThan(ParseSimple(constraintVersion.Major(), constraintVersion.Minor()+1, 0)), nil
	}

	// Wildcard
	if strings.Contains(constraint, "*") {
		if constraint == "*" {
			// Any version matches
			return true, nil
		}
		parts := strings.Split(constraint, ".")
		if len(parts) == 2 && parts[1] == "*" {
			maj, err := strconv.Atoi(parts[0])
			if err != nil {
				return false, err
			}
			return v.GreaterThanOrEqual(ParseSimple(maj, 0, 0)) && v.LessThan(ParseSimple(maj+1, 0, 0)), nil
		}

		if len(parts) == 3 && parts[2] == "*" {
			maj, err := strconv.Atoi(parts[0])
			if err != nil {
				return false, err
			}
			min, err := strconv.Atoi(parts[1])
			if err != nil {
				return false, err
			}
			return v.GreaterThanOrEqual(ParseSimple(maj, min, 0)) && v.LessThan(ParseSimple(maj, min+1, 0)), nil
		}
	}

	// Handle others
	operators := []string{">=", "<=", ">", "<", "==", "!=", "="}
	for _, operator := range operators {
		if strings.HasPrefix(constraint, operator) {
			constraintVersionString := strings.TrimSpace(constraint[len(operator):])
			constraintVersion, err := ParseVersionFromRegex(constraintVersionString, RegexpSimple)
			if err != nil {
				return false, err
			}
			switch operator {
			case ">=":
				return v.GreaterThanOrEqual(constraintVersion), nil
			case ">":
				return v.GreaterThan(constraintVersion), nil
			case "<=":
				return v.LessThanOrEqual(constraintVersion), nil
			case "<":
				return v.LessThan(constraintVersion), nil
			case "==", "=":
				return v.Equals(constraintVersion), nil
			case "!=":
				return !v.Equals(constraintVersion), nil
			}
		}
	}

	// Exact version
	constraintVersion, err := ParseVersionFromRegex(constraint, RegexpSimple)
	if err == nil {
		return v.Equals(constraintVersion), nil
	}

	return false, fmt.Errorf("invalid constraint: %s", constraint)
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

func (a *Version) GreaterThanOrEqual(b *Version) bool {
	return a.CompareTo(b) >= 0
}

func (a *Version) LessThan(b *Version) bool {
	return a.CompareTo(b) == -1
}

func (a *Version) LessThanOrEqual(b *Version) bool {
	return a.CompareTo(b) <= 0
}

func (a *Version) Equals(b *Version) bool {
	return a.CompareTo(b) == 0
}

func Sort(versions []*Version) {
	slices.SortStableFunc(versions, Compare)
}

// Gets the maximum version which complies to a given version of a list of objects that contain a version.
func FindMaxGeneric[T any](versions []T, getFunc func(x T) *Version, referenceVersion *Version, onlyWithoutStringValues bool) T {
	var maxVersion *Version = nil
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
			if maxVersion == nil ||
				version.GreaterThan(maxVersion) ||
				(version.Equals(maxVersion) && version.SegmentCount(true) > maxVersion.SegmentCount(true)) {
				maxVersion = version
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
func ParseSimple(parts ...any) *Version {
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
			// Convert the value to string
			str := fmt.Sprintf("%v", v)
			segmentsToAdd = append(segmentsToAdd, buildSegmentFromString(str))
		}
		// Add all the new segments
		version.Segments = append(version.Segments, segmentsToAdd...)
	}
	return version
}

// Parses the given version string with the regexp into the version object. Throws a panic on error.
func MustParseVersionFromRegex(versionString string, versionRegexp *regexp.Regexp) *Version {
	return must(ParseVersionFromRegex(versionString, versionRegexp))
}

// Parses the given version string with the regexp into the version object.
func ParseVersionFromRegex(versionString string, versionRegexp *regexp.Regexp) (*Version, error) {
	// Initially set the raw value to the full version string
	rawValue := versionString

	// Find the named matches of the parts
	matchMap := findNamedMatches(versionRegexp, versionString, true)
	if matchMap == nil {
		return nil, fmt.Errorf("failed parsing the version %s: %w", versionString, ErrNoMatch)
	}

	// Build a map with index and the segments
	insertMap := map[int]VersionSegment{}
	for k, v := range matchMap {
		// Special handling for the raw group
		if k == "raw" {
			// Set the raw value to the value of the group
			rawValue = v
			continue
		}
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
	parsedVersion := &Version{Raw: rawValue}
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

// Find all parts represented by capturing groups. Either with their name or otherwise names them with <p<index>>.
func findNamedMatches(regex *regexp.Regexp, str string, includeNotMatchedOptional bool) map[string]string {
	match := regex.FindStringSubmatchIndex(str)
	if match == nil {
		// No matches
		return nil
	}
	subexpNames := regex.SubexpNames()
	results := map[string]string{}
	indexAdjustment := 0 // Holds an index adjustment in case a raw value was found (and should not be counted)
	// Loop thru the subexp names (skipping the first empty one)
	for i, name := range (subexpNames)[1:] {
		if name == "raw" {
			indexAdjustment = -1
		}
		if name == "" {
			// No name, so automatically give it a name
			name = fmt.Sprintf("p%d", (i + 1 + indexAdjustment))
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
