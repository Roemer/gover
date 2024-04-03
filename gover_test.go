package gover

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSimple(t *testing.T) {
	assert := assert.New(t)

	{
		v1 := ParseSimple(1, 2, 3)
		assert.Len(v1.Segments, 3)
		assert.Equal(1, v1.Segments[0].Number)
		assert.Equal(2, v1.Segments[1].Number)
		assert.Equal(3, v1.Segments[2].Number)
	}

	{
		v2 := ParseSimple(1, "hello", 3)
		assert.Len(v2.Segments, 3)
		assert.Equal(1, v2.Segments[0].Number)
		assert.Equal("hello", v2.Segments[1].Text)
		assert.Equal(3, v2.Segments[2].Number)
	}

	{
		v3 := ParseSimple(strings.Split("1.2.3", "."))
		assert.Len(v3.Segments, 3)
		assert.Equal(1, v3.Segments[0].Number)
		assert.Equal(2, v3.Segments[1].Number)
		assert.Equal(3, v3.Segments[2].Number)
	}
}

func TestJavaVersioning(t *testing.T) {
	assert := assert.New(t)

	// d.d.d(_d)-d
	reg := regexp.MustCompile(`^(?P<d1>\d+)\.(?P<d2>\d+)\.(?P<d3>\d+)(?:_(?P<d4>\d+))?-(?P<d5>\d+)$`)

	versionListSorted := []string{
		"1.8.0_332-1",
		"1.8.0_345-2",
		"1.8.0_372-1",
		"1.8.0_372-2",
		"1.8.0_372-3",
		"1.8.0_402-1",
		"11.0.15-1",
		"11.0.17-10",
		"11.0.19-1",
		"11.0.19-2",
		"17.0.3-1",
		"17.0.4-2",
		"17.0.7-1",
		"17.0.8-3",
		"17.0.8-5",
		"17.0.9-1",
		"17.0.9-2",
		"21.0.0-1",
		"21.0.1-1",
		"21.0.1-2",
		"21.0.1-3",
		"21.0.1-4",
		"21.0.2-1",
		"21.0.2-50",
	}

	// Create a randomized list
	versionListRandomized := make([]string, len(versionListSorted))
	_ = copy(versionListRandomized, versionListSorted)
	rand.Shuffle(len(versionListRandomized), func(i, j int) {
		versionListRandomized[i], versionListRandomized[j] = versionListRandomized[j], versionListRandomized[i]
	})

	// Build the list with parsed versions
	versionsSorted := []*Version{}
	versionsRandomized := []*Version{}
	for _, versionString := range versionListRandomized {
		versionsSorted = append(versionsSorted, MustParseVersionFromRegex(versionString, reg))
		versionsRandomized = append(versionsRandomized, MustParseVersionFromRegex(versionString, reg))
	}
	Sort(versionsSorted)

	// Test the sorting
	for i, version := range versionsSorted {
		assert.Equal(versionListSorted[i], version.Original)
	}

	// Test FindMax
	assert.Equal(FindMax(versionsRandomized, EmptyVersion, true).Original, "21.0.2-50")
	assert.Equal(FindMax(versionsRandomized, ParseSimple(21), true).Original, "21.0.2-50")
	assert.Equal(FindMax(versionsRandomized, ParseSimple(21, 0, 1), true).Original, "21.0.1-4")
	assert.Equal(FindMax(versionsRandomized, ParseSimple(11, 0, 19), true).Original, "11.0.19-2")
	assert.Equal(FindMax(versionsRandomized, ParseSimple(17, 0, 8), true).Original, "17.0.8-5")
}

func TestMax(t *testing.T) {
	assert := assert.New(t)

	versionList := []string{
		"2.0-alpha-1",
		"2.0-alpha-2",
		"2.0-alpha-3",
		"2.0-beta-1",
		"2.0-beta-2",
		"2.0-beta-3",
		"2.0",
		"2.0.1",
		"2.0.2",
		"2.0.3",
		"2.0.4",
		"2.1.0-M1",
		"3.0-alpha-1",
		"3.0-alpha-2",
		"3.0-alpha-3",
		"3.0-alpha-4",
		"3.0-alpha-5",
		"3.0-alpha-6",
		"3.0-alpha-7",
		"3.0-beta-1",
		"3.0-beta-2",
		"3.0-beta-3",
		"3.0",
		"3.5.0-alpha-1",
		"3.5.0-beta-1",
		"3.5.0",
		"3.5.2",
		"3.5.3",
		"3.5.4",
		"3.6.0",
		"4.0.0-alpha-2",
		"4.0.0-alpha-3",
	}

	reg := regexp.MustCompile(`^(?P<d1>\d+)\.(?P<d2>\d+)(?:\.(?P<d3>\d+))?(?:-(?P<s4>[^-]+))?(?:-(?P<d5>\d+))?$`)

	allVersions := []*Version{}
	for _, v := range versionList {
		parsedVersion := MustParseVersionFromRegex(v, reg)
		allVersions = append(allVersions, parsedVersion)
	}

	assert.Equal(FindMax(allVersions, EmptyVersion, false).Original, "4.0.0-alpha-3")
	assert.Equal(FindMax(allVersions, EmptyVersion, true).Original, "3.6.0")
}

func TestAutoNumbering(t *testing.T) {
	assert := assert.New(t)
	reg := regexp.MustCompile(`^(\d+)(?:\.(\d+))?(?:-(.+))?$`)
	versionList := []string{
		"1",
		"2.0-a",
		"2.0-b",
		"2.0",
		"2.5-rc",
		"2.5",
	}

	allVersions := []*Version{}
	for _, v := range versionList {
		parsedVersion := MustParseVersionFromRegex(v, reg)
		allVersions = append(allVersions, parsedVersion)
	}

	assert.Len(allVersions, 6)
	{
		checkVersion := allVersions[0]
		assert.Len(checkVersion.Segments, 3)
		assert.Equal(1, checkVersion.Segments[0].Number)
		assert.True(checkVersion.Segments[1].IsNotDefined)
		assert.True(checkVersion.Segments[2].IsNotDefined)
	}
	{
		checkVersion := allVersions[1]
		assert.Len(checkVersion.Segments, 3)
		assert.Equal(2, checkVersion.Segments[0].Number)
		assert.Equal(0, checkVersion.Segments[1].Number)
		assert.Equal("a", checkVersion.Segments[2].Text)
	}
	{
		checkVersion := allVersions[2]
		assert.Len(checkVersion.Segments, 3)
		assert.Equal(2, checkVersion.Segments[0].Number)
		assert.Equal(0, checkVersion.Segments[1].Number)
		assert.Equal("b", checkVersion.Segments[2].Text)
	}
	{
		checkVersion := allVersions[3]
		assert.Len(checkVersion.Segments, 3)
		assert.Equal(2, checkVersion.Segments[0].Number)
		assert.Equal(0, checkVersion.Segments[1].Number)
		assert.True(checkVersion.Segments[2].IsNotDefined)
	}
	{
		checkVersion := allVersions[4]
		assert.Len(checkVersion.Segments, 3)
		assert.Equal(2, checkVersion.Segments[0].Number)
		assert.Equal(5, checkVersion.Segments[1].Number)
		assert.Equal("rc", checkVersion.Segments[2].Text)
	}
	{
		checkVersion := allVersions[5]
		assert.Len(checkVersion.Segments, 3)
		assert.Equal(2, checkVersion.Segments[0].Number)
		assert.Equal(5, checkVersion.Segments[1].Number)
		assert.True(checkVersion.Segments[2].IsNotDefined)
	}
}

func TestError(t *testing.T) {
	assert := assert.New(t)

	reg := regexp.MustCompile(`^a(?P<d1>\d+)$`)

	vNoError, err := ParseVersionFromRegex("a1", reg)
	assert.NotNil(vNoError)
	assert.Nil(err)

	vError, err := ParseVersionFromRegex("b1", reg)
	assert.Nil(vError)
	assert.NotNil(err)
	assert.ErrorIs(err, ErrNoMatch)
}
