package match

import (
	"reflect"
	"regexp"
)

type MatchKey int

type matchItem struct {
	pattern interface{}
	action  func() interface{}
}

// PatternChecker is func for checking pattern.
type PatternChecker func(pattern interface{}, value interface{}) bool

var (
	registeredMatchers []PatternChecker
)

const (
	// ANY is the pattern which allows any value.
	ANY MatchKey = 0
	// HEAD is the pattern for start element of silce.
	HEAD MatchKey = 1
	// TAIL is the pattern for end element(s) of slice.
	TAIL MatchKey = 2
)

type oneOfContainer struct {
	items []interface{}
}

// OneOf defines the pattern where at least one item matches.
func OneOf(items ...interface{}) oneOfContainer {
	return oneOfContainer{items}
}

// Matcher struct
type Matcher struct {
	value      interface{}
	matchItems []matchItem
}

// Match function takes a value for matching and
func Match(val interface{}) *Matcher {
	matchItems := []matchItem{}
	return &Matcher{val, matchItems}
}

// When function adds new pattern for checking matching.
// If pattern matched with value the func will be called.
func (matcher *Matcher) When(val interface{}, fun func() interface{}) *Matcher {
	newMatchItem := matchItem{val, fun}
	matcher.matchItems = append(matcher.matchItems, newMatchItem)

	return matcher
}

// RegisterMatcher register custom pattern.
func RegisterMatcher(pattern PatternChecker) {
	registeredMatchers = append(registeredMatchers, pattern)
}

// Result returns the result value of matching process.
func (matcher *Matcher) Result() (bool, interface{}) {
	for _, mi := range matcher.matchItems {
		matched := matchValue(mi.pattern, matcher.value)
		if matched {
			return true, mi.action()
		}
	}

	return false, nil
}

func matchValue(pattern interface{}, value interface{}) bool {
	simpleTypes := []reflect.Kind{reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128,
	}

	valueKind := reflect.TypeOf(value).Kind()
	valueIsSimpleType := containsKind(simpleTypes, valueKind)

	for _, registerMatcher := range registeredMatchers {
		if registerMatcher(pattern, value) {
			return true
		}
	}

	if (valueIsSimpleType) && value == pattern {
		return true
	}

	patternType := reflect.TypeOf(pattern)
	patternKind := patternType.Kind()

	if (valueKind == reflect.Slice || valueKind == reflect.Array) &&
		patternKind == reflect.Slice &&
		matchSlice(pattern, value) {

		return true
	}

	if patternKind == reflect.Func && patternType.NumIn() == 1 &&
		matchStruct(patternType.In(0), value) {
		return true
	}

	if valueKind == reflect.Map &&
		patternKind == reflect.Map &&
		matchMap(pattern, value) {

		return true
	}

	if valueKind == reflect.String {
		if patternKind == reflect.String {
			if pattern == value {
				return true
			}
		}

		reg, ok := pattern.(*regexp.Regexp)
		if ok {
			if matchRegexp(reg, value) {
				return true
			}
		}
	}

	if valueKind == reflect.Struct {
		if patternKind == reflect.Struct {
			if value == pattern {
				return true
			}
		}
	}

	return false
}

func matchSlice(pattern interface{}, value interface{}) bool {
	patternSlice := reflect.ValueOf(pattern)
	patternSliceLen := patternSlice.Len()

	valueSlice := reflect.ValueOf(value)
	valueSliceLen := valueSlice.Len()

	if patternSliceLen > 0 && patternSlice.Index(0).Interface() == HEAD {
		if valueSliceLen == 0 {
			return false
		}

		patternSliceVal := patternSlice.Slice(1, patternSliceLen)
		patternSliceLen = patternSliceVal.Len()
		patternSliceInterface := patternSliceVal.Interface()

		for i := 0; i < valueSliceLen-patternSliceLen+1; i++ {
			isMatched := matchSubSlice(patternSliceInterface, valueSlice.Slice(i, valueSliceLen).Interface())
			if isMatched {
				return true
			}
		}

		return false
	}

	return matchSubSlice(pattern, value)
}

func matchSubSlice(pattern interface{}, value interface{}) bool {
	patternSlice := reflect.ValueOf(pattern)
	valueSlice := reflect.ValueOf(value)

	patternSliceLength := patternSlice.Len()
	valueSliceLength := valueSlice.Len()

	if patternSliceLength == 0 || valueSliceLength == 0 {
		if patternSliceLength == valueSliceLength {
			return true
		}
		return false
	}

	patternSliceMaxIndex := patternSliceLength - 1
	valueSliceMaxIndex := valueSliceLength - 1
	oneOfContainerType := reflect.TypeOf(oneOfContainer{})

	for i := 0; i < max(patternSliceLength, valueSliceLength); i++ {
		currPatternIndex := min(i, patternSliceMaxIndex)
		currValueIndex := min(i, valueSliceMaxIndex)

		currPattern := patternSlice.Index(currPatternIndex).Interface()
		currValue := valueSlice.Index(currValueIndex).Interface()

		if currPattern == HEAD {
			panic("HEAD can only be in first position of a pattern.")
		} else if currPattern == TAIL {
			if patternSliceMaxIndex > i {
				panic("TAIL must me in last position of the pattern.")
			} else {
				break
			}
		} else if reflect.TypeOf(currPattern).AssignableTo(oneOfContainerType) {
			oneOfContainerPatternInstance := currPattern.(oneOfContainer)
			matched := false
			for _, item := range oneOfContainerPatternInstance.items {
				if matchValue(item, currValue) {
					matched = true
					break
				}
			}

			if !matched {
				return false
			}
		} else {
			if currPattern != ANY && !matchValue(currPattern, currValue) {
				return false
			}
		}
	}

	return true
}

func matchStruct(patternType reflect.Type, value interface{}) bool {
	if patternType.AssignableTo(reflect.TypeOf(value)) {
		return true
	}

	return false
}

func matchMap(pattern interface{}, value interface{}) bool {
	patternMap := reflect.ValueOf(pattern)
	valueMap := reflect.ValueOf(value)

	stillUsablePatternKeys := patternMap.MapKeys()
	stillUsableValueKeys := valueMap.MapKeys()

	for _, pKey := range patternMap.MapKeys() {
		if !containsValue(stillUsablePatternKeys, pKey) {
			continue
		}
		pVal := patternMap.MapIndex(pKey)
		matchedLeftAndRight := false

		for _, vKey := range valueMap.MapKeys() {
			if !containsValue(stillUsableValueKeys, vKey) {
				continue
			}

			if !containsValue(stillUsablePatternKeys, pKey) {
				continue
			}

			vVal := valueMap.MapIndex(vKey)
			keyMatched := pKey.Interface() == vKey.Interface()
			if keyMatched {
				valueMatched := matchValue(pVal.Interface(), vVal.Interface()) || pVal.Interface() == ANY
				if valueMatched {
					matchedLeftAndRight = true
					removeValue(stillUsablePatternKeys, pKey)
					removeValue(stillUsableValueKeys, vKey)
				}
			}
		}

		if !matchedLeftAndRight {
			return false
		}
	}

	return true
}

func matchRegexp(regexp *regexp.Regexp, value interface{}) bool {
	valueStr := value.(string)

	return regexp.MatchString(valueStr)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

func removeValue(vals []reflect.Value, val reflect.Value) []reflect.Value {
	indexOf := -1
	for index, v := range vals {
		if val.Interface() == v.Interface() {
			indexOf = index
			break
		}
	}

	vals[indexOf] = vals[len(vals)-1]
	vals = vals[:len(vals)-1]

	return vals
}

func containsValue(vals []reflect.Value, val reflect.Value) bool {
	for _, v := range vals {
		if val.Interface() == v.Interface() {
			return true
		}
	}
	return false
}

func containsKind(vals []reflect.Kind, val reflect.Kind) bool {
	for _, v := range vals {
		if val == v {
			return true
		}
	}
	return false
}
