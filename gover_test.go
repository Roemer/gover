package gover

import (
	"math/rand"
	"regexp"
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
	versions := []*Version{}
	for _, versionString := range versionListRandomized {
		versions = append(versions, MustParseVersionFromRegex(versionString, reg))
	}
	Sort(versions)

	// Test the sorting
	for i, version := range versions {
		assert.Equal(versionListSorted[i], version.Original)
	}

	// Test FindMax
	assert.Equal(FindMax(versions, EmptyVersion, true).Original, "21.0.2-50")
	assert.Equal(FindMax(versions, ParseSimple(21), true).Original, "21.0.2-50")
	assert.Equal(FindMax(versions, ParseSimple(21, 0, 1), true).Original, "21.0.1-4")
	assert.Equal(FindMax(versions, ParseSimple(11, 0, 19), true).Original, "11.0.19-2")
	assert.Equal(FindMax(versions, ParseSimple(17, 0, 8), true).Original, "17.0.8-5")
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
