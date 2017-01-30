// Lunar Logo: clean, minimal scripting language based on Logo and Lua.
package main

import (
	"fmt"
	"strings"
	"strconv"
	"regexp"
	"os"
	"bufio"
	"math"
	"math/rand"
	"time"
)

var Ins = os.Stdin
var Outs = os.Stdout
var Errs = os.Stderr

var intre = regexp.MustCompile(`^-?[[:digit:]]+$`)
var splitre = regexp.MustCompile(`[[:space:]]+`)
var spacere = regexp.MustCompile(`^[[:space:]]+$`)
var digitre = regexp.MustCompile(`^[[:digit:]]+$`)
var alphare = regexp.MustCompile(`^[[:alpha:]]+$`)
var alnumre = regexp.MustCompile(`^[[:alnum:]]+$`)

type List []interface{}
type Dict map[string]interface{}

type Scope struct {
	Names Dict
	Parent *Scope
	
	continuing bool
	breaking bool
	returning bool
}

type Builtin struct {
	Arity int
	Code func (*Scope, ...interface{}) (interface{}, error)
}

type Closure struct {
	Arglist []string
	Code List
	*Scope
}

type Error struct {
	Data interface{}
}

func (self Error) Error() string {
	return fmt.Sprint(self.Data)
}

func (self List) Len() int { return len(self) }
func (self List) Swap(a, b int) { self[a], self[b] = self[b], self[a] }
func (self List) Less(a, b int) bool {
	switch item1 := self[a].(type) {
	case bool:
		switch item2 := self[b].(type) {
			case bool: return (!item1) && item2
			default: panic(Error{fmt.Sprintf(
				"Can't compare %T to %T.", item1, item2)})
		}
	case int:
		switch item2 := self[b].(type) {
			case int: return item1 < item2
			case float64: return float64(item1) < item2
			default: panic(Error{fmt.Sprintf(
				"Can't compare %T to %T.", item1, item2)})
		}
	case float64:
		switch item2 := self[b].(type) {
			case int: return item1 < float64(item2)
			case float64: return item1 < item2
			default: panic(Error{fmt.Sprintf(
				"Can't compare %T to %T.", item1, item2)})
		}
	case string:
		switch item2 := self[b].(type) {
			case string: return item1 < item2
			default: panic(Error{fmt.Sprintf(
				"Can't compare %T to %T.", item1, item2)})
		}
	default:
		panic(Error{fmt.Sprintf(
			"No comparisons defined on %T.", item1)})
	}
}

// Equal complements sort.Interface to enable all comparison operators.
func (self List) Equal(a, b int) bool {
	switch item1 := self[a].(type) {
	case int:
		switch item2 := self[b].(type) {
			case int: return item1 == item2
			case float64: return float64(item1) == item2
			default: panic(Error{fmt.Sprintf(
				"Can't compare %T to %T.", item1, item2)})
		}
	case float64:
		switch item2 := self[b].(type) {
			case int: return item1 == float64(item2)
			case float64: return item1 == item2
			default: panic(Error{fmt.Sprintf(
				"Can't compare %T to %T.", item1, item2)})
		}
	default:
		return self[a] == self[b]
	}
}

func (self *Scope) Get(name string) (interface{}, error) {
	if value, ok := self.Names[name]; ok {
		return value, nil
	} else if self.Parent != nil {
		return self.Parent.Get(name)
	} else {
		return nil, Error{"Undefined variable: " + name}
	}
}

func (self *Scope) SafeGet(name string, fallback interface{}) interface{} {
	if value, ok := self.Names[name]; ok {
		return value
	} else if self.Parent != nil {
		return self.Parent.SafeGet(name, fallback)
	} else {
		return fallback
	}
}

func (self *Scope) Put(name string, value interface{}) {
	if _, ok := self.Names[name]; ok {
		self.Names[name] = value
	} else if self.Parent != nil {
		self.Parent.Put(name, value)
	} else {
		self.Names[name] = value
	}
}

func (self *Closure) Apply(args ...interface{})  (interface{}, error) {
	locals := Scope{Names: Dict{}, Parent: self.Scope}
	if len(self.Arglist) != len(args) {
		return nil, Error{fmt.Sprintf(
			"%d arguments passed to function expecting %d.",
			len(args), len(self.Arglist))}
	}
	for i, n := range(self.Arglist) {
		locals.Names[n] = args[i]
	}
	return Run(self.Code, &locals)
}

func EvalNext(code List, cursor int, scope *Scope) (interface{}, int, error) {
	collectArgs := func (num int, msg string) (List, error) {
		args := make(List, num)
		for i := 0; i < num; i++ {
			if cursor >= len(code) {
				return args, Error{msg}
			}
			tmp, csr, err := EvalNext(code, cursor, scope)
			if err != nil {
				return args, err
			}
			args[i] = tmp
			cursor = csr
		}
		return args, nil
	}
	
	value := code[cursor]
	
	switch value := value.(type) {
	case Builtin:
		cursor++
		args, err := collectArgs(value.Arity, "Not enough arguments.")
		if err != nil {
			return nil, cursor, err
		}
		tmp, err := value.Code(scope, args...)
		return tmp, cursor, err
	case string:
		if value[0] == ':' {
			// Expect name to be already lowercased.
			tmp, err := scope.Get(value[1:])
			return tmp, cursor + 1, err
		} else if value == "do" {
			return ScanBlock(code, cursor + 1)
		} else {
			closure := scope.SafeGet(
				strings.ToLower(value), value)
			if closure, ok := closure.(Closure); ok {
				cursor++
				args, err := collectArgs(
					len(closure.Arglist),
					"Not enough arguments to " +
						strings.ToLower(value))
				if err != nil {
					return nil, cursor, err
				}
				tmp, err := closure.Apply(args...)
				return tmp, cursor, err
			} else {
				return value, cursor + 1, nil
			}
		}
	default:
		return value, cursor + 1, nil
	}
	return nil, 0, nil
}

func ScanBlock(code List, cursor int) (List, int, error) {
	block := make(List, 0, len(code) - cursor)
	for code[cursor] != "end" {
		if code[cursor] == "do" {
			tmp, csr, err := ScanBlock(code, cursor + 1)
			if err != nil {
				return block, csr, err
			}
			block = append(block, tmp)
			cursor = csr
		} else {
			block = append(block, code[cursor])
			cursor++
		}
		if cursor >= len(code) {
			return block, cursor, Error{
				"Unexpected end of input in block."}
		}
	}
	return block, cursor + 1, nil
}

func Parse(words []string, context map[string]Builtin) (List, error) {
	code := make([]interface{}, 0, len(words))
	var buf []string = nil
	in_list := false
	for _, i := range(words) {
		lower := strings.ToLower(i)
		if in_list {
			if strings.HasSuffix(i, "]") {
				if len(i) > 1 {
					buf = append(buf, i[:len(i) - 1])
				}
				code = append(code, buf)
				in_list = false
			} else {
				buf = append(buf, i)
			}
		} else if i == "[]" {
			code = append(code, make(List, 0))
		} else if strings.HasPrefix(i, "[") {
			if strings.HasSuffix(i, "]") {
				code = append(code, []string{i[1:len(i) - 1]})
			} else {
				buf = make([]string, 0)
				if len(i) > 1 {
					buf = append(buf, i[1:])
				}
				in_list = true
			}
		} else if strings.HasPrefix(i, "--") {
			break
		} else if strings.HasPrefix(i, ":") {
			code = append(code, lower)
		} else if lower  == "do" || lower == "end" {
			code = append(code, lower)
		} else if lower == "true" {
			code = append(code, true)
		} else if lower == "false" {
			code = append(code, false)
		} else if lower == "nil" {
			code = append(code, nil)
		} else if proc, ok := context[lower]; ok {
			code = append(code, proc)
		} else if intre.MatchString(i) {
			value, err := strconv.Atoi(i)
			if err == nil {
				code = append(code, value)
			} else {
				code = append(code, 0)
			}
		} else {
			value, err := strconv.ParseFloat(i, 64)
			if err == nil {
				code = append(code, value)
			} else {
				code = append(code, i)
			}
		}
	}
	if in_list {
		return List(code), Error{"Unclosed list at end of line."}
	} else {
		return List(code), nil
	}
}

//Underlies most other control structures.
func Run(code List, scope *Scope) (interface{}, error) {
	cursor := 0
	for cursor < len(code) {
		value, csr, err := EvalNext(code, cursor, scope)
		if err != nil {
			return nil, err
		} else if scope.continuing || scope.breaking {
			return nil, nil
		} else if scope.returning {
			return value, nil
		} else if value != nil {
			return value, Error{
				"You don't say what to do with: " +
					fmt.Sprint(value)}
		}
		cursor = csr
	}
	return nil, nil
}

// Underlies while, ifelse and the command line.
func Results(code List, scope *Scope) (List, error) {
	values := make([]interface{}, 0, len(code))
	cursor := 0
	for cursor < len(code) {
		val, csr, err := EvalNext(code, cursor, scope)
		if err != nil {
			return List(values), err
		} else if scope.returning {
			return List{val}, nil
		} else if scope.breaking || scope.continuing {
			break
		}
		values = append(values, val)
		cursor = csr
	}
	return List(values), nil
}

func Load(fn string, ctx map[string]Builtin, s *Scope) (interface{}, error) {
	code := make([]interface{}, 0)
	file, err := os.Open(fn)
	if err != nil { return nil, err }
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 { continue }
		words := splitre.Split(line, -1)
		tokens, err := Parse(words, ctx)
		if err != nil { return nil, err }
		code = append(code, tokens...)
	}
	if scanner.Err() == nil {
		return Run(List(code), s)
	} else {
		return nil, scanner.Err()
	}
}

// For loop; the variable is always treated as local.
func For(v string, i, l, p float64, code List, s *Scope) (interface{}, error) {
	v = strings.ToLower(v)
	s.Names[v] = i
	if l >= i {
		for i <= l {
			value, err := Run(code, s)
			if err != nil {
				return nil, err
			} else if s.returning {
				return value, nil
			} else if s.continuing {
				s.continuing = false
			} else if s.breaking {
				s.breaking = false
				break
			}
			i += p
			s.Names[v] = i
		}
	} else {
		for i >= l {
			value, err := Run(code, s)
			if err != nil {
				return nil, err
			} else if s.returning {
				return value, nil
			} else if s.continuing {
				s.continuing = false
			} else if s.breaking {
				s.breaking = false
				break
			}
			i += p
			s.Names[v] = i
		}
	}
	return nil, nil
}

func First(value interface{}) (interface{}, error) {
	switch seq := value.(type) {
	case List:
		if len(seq) > 0 {
			return seq[0], nil
		} else {
			return nil, Error{"First got an empty list."}
		}
	case []string:
		if len(seq) > 0 {
			return seq[0], nil
		} else {
			return nil, Error{"First got empty literal list."}
		}
	case string:
		if len(seq) > 0 {
			return seq[0], nil
		} else {
			return nil, Error{"First got an empty string."}
		}
	default:
		return nil, Error{
			"First expects a sequence, got: " + fmt.Sprint(value)}
	}
}

func Last(value interface{}) (interface{}, error) {
	switch seq := value.(type) {
	case List:
		if len(seq) > 0 {
			return seq[len(seq) - 1], nil
		} else {
			return nil, Error{"Last got an empty list."}
		}
	case []string:
		if len(seq) > 0 {
			return seq[len(seq) - 1], nil
		} else {
			return nil, Error{"Last got empty literal list."}
		}
	case string:
		if len(seq) > 0 {
			return seq[len(seq) - 1], nil
		} else {
			return nil, Error{"Last got an empty string."}
		}
	default:
		return nil, Error{
			"Last expects a sequence, got: " + fmt.Sprint(value)}
	}
}

func ButFirst(value interface{}) (interface{}, error) {
	switch seq := value.(type) {
	case List:
		if len(seq) > 0 {
			return seq[1:], nil
		} else {
			return nil, Error{"ButFirst got an empty list."}
		}
	case []string:
		if len(seq) > 0 {
			return seq[1:], nil
		} else {
			return nil, Error{"ButFirst got empty literal list."}
		}
	case string:
		if len(seq) > 0 {
			return seq[1:], nil
		} else {
			return nil, Error{"ButFirst got an empty string."}
		}
	default:
		return nil, Error{
			"ButFirst expects a sequence, got: " +
				fmt.Sprint(value)}
	}
}

func ButLast(value interface{}) (interface{}, error) {
	switch seq := value.(type) {
	case List:
		if len(seq) > 0 {
			return seq[0:len(seq) - 1], nil
		} else {
			return nil, Error{"ButLast got an empty list."}
		}
	case []string:
		if len(seq) > 0 {
			return seq[0:len(seq) - 1], nil
		} else {
			return nil, Error{"ButLast got empty literal list."}
		}
	case string:
		if len(seq) > 0 {
			return seq[0:len(seq) - 1], nil
		} else {
			return nil, Error{"ButLast got an empty string."}
		}
	default:
		return nil, Error{
			"ButLast expects a sequence, got: " +
				fmt.Sprint(value)}
	}
}

func Pick(value interface{}) (interface{}, error) {
	switch seq := value.(type) {
	case List:
		if len(seq) > 0 {
			return seq[rand.Intn(len(seq))], nil
		} else {
			return nil, Error{"Pick got an empty list."}
		}
	case []string:
		if len(seq) > 0 {
			return seq[rand.Intn(len(seq))], nil
		} else {
			return nil, Error{"Pick got empty literal list."}
		}
	case string:
		if len(seq) > 0 {
			return seq[rand.Intn(len(seq))], nil
		} else {
			return nil, Error{"Pick got an empty string."}
		}
	default:
		return nil, Error{
			"Pick expects a sequence, got: " + fmt.Sprint(value)}
	}
}

func ToBool(input interface{}) bool {
	switch input := input.(type) {
		case bool: return input
		case int: return input != 0
		case float64: return input != 0
		default: panic(Error{fmt.Sprintf(
			"Can't convert %#v to bool.", input)})
	}
}

func ToString(input interface{}) string {
	switch input := input.(type) {
		case string: return string(input)
		default: return fmt.Sprint(input)
	}
}

func ParseFloat(input interface{}) float64 {
	switch input := input.(type) {
		case float64: return float64(input)
		case int: return float64(input)
		case string:
			value, err := strconv.ParseFloat(input, 64)
			if err == nil {
				return value
			} else {
				return math.NaN()
			}
		default: return math.NaN()
	}
}

func ParseInt(input interface{}) int {
	switch input := input.(type) {
		case float64: return int(input)
		case int: return int(input)
		case string:
			value, err := strconv.Atoi(input)
			if err == nil {
				return value
			} else {
				return int(math.NaN())
			}
		default: panic(Error{fmt.Sprintf(
			"Can't convert %#v to int.", input)})
	}
}

var Procedures = map[string]Builtin {
	"run": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		if code, ok := a[0].(List); ok {
			return Run(code, s)
		} else {
			return nil, Error{
				"Run expects a list, found: " +
				fmt.Sprint(a[0])}
		}
	}},
	"results": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		if code, ok := a[0].(List); ok {
			return Results(code, s)
		} else {
			return nil, Error{
				"Results expects a list, found: " +
				fmt.Sprint(a[0])}
		}
	}},
	"ignore": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		return nil, nil
	}},

	"break": {0, func (s *Scope, a ...interface{}) (interface{}, error) {
		s.breaking = true
		return nil, nil
	}},
	"continue": {0,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		s.continuing = true
		return nil, nil
	}},
	"return": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		s.returning = true
		return a[0], nil
	}},
	
	"print": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		fmt.Fprintln(Outs, a[0])
		return nil, nil
	}},
	"type": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		fmt.Fprint(Outs, a[0])
		return nil, nil
	}},
	"show": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		fmt.Fprintf(Outs, "%#v\n", a[0])
		return nil, nil
	}},
	
	"make": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		varname := ToString(a[0])
		s.Put(strings.ToLower(varname), a[1])
		return nil, nil
	}},
	"localmake": {2,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		varname := ToString(a[0])
		s.Names[strings.ToLower(varname)] = a[1]
		return nil, nil
	}},
	"thing": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		varname := ToString(a[0])
		return s.Get(strings.ToLower(varname))
	}},
	
	"for": {5, func (s *Scope, a ...interface{}) (interface{}, error) {
		varname := ToString(a[0])
		init := ParseFloat(a[1])
		limit := ParseFloat(a[2])
		step := ParseFloat(a[3])
		code := a[4].(List)
		return For(varname, init, limit, step, code, s)
	}},
	
	"add": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		switch t1 := a[0].(type) {
		case int:
			switch t2 := a[1].(type) {
				case int: return t1 + t2, nil
				case float64: return float64(t1) + t2, nil
				default: return math.NaN(), nil
			}
		case float64:
			switch t2 := a[1].(type) {
				case int: return t1 + float64(t2), nil
				case float64: return t1 + t2, nil
				default: return math.NaN(), nil
			}
		default:
			return math.NaN(), nil
		}
	}},
	"sub": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		switch t1 := a[0].(type) {
		case int:
			switch t2 := a[1].(type) {
				case int: return t1 - t2, nil
				case float64: return float64(t1) - t2, nil
				default: return math.NaN(), nil
			}
		case float64:
			switch t2 := a[1].(type) {
				case int: return t1 - float64(t2), nil
				case float64: return t1 - t2, nil
				default: return math.NaN(), nil
			}
		default:
			return math.NaN(), nil
		}
	}},
	"mul": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		switch t1 := a[0].(type) {
		case int:
			switch t2 := a[1].(type) {
				case int: return t1 * t2, nil
				case float64: return float64(t1) * t2, nil
				default: return math.NaN(), nil
			}
		case float64:
			switch t2 := a[1].(type) {
				case int: return t1 * float64(t2), nil
				case float64: return t1 * t2, nil
				default: return math.NaN(), nil
			}
		default:
			return math.NaN(), nil
		}
	}},
	"div": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		return ParseFloat(a[0]) / ParseFloat(a[1]), nil
	}},
	"mod": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		return ParseInt(a[0]) % ParseInt(a[1]), nil
	}},
	"pow": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		return math.Pow(ParseFloat(a[0]), ParseFloat(a[1])), nil
	}},
	"abs": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		switch n := a[0].(type) {
			case int:
				if n < 1 {
					return -n, nil
				} else {
					return n, nil
				}
			case float64: return math.Abs(n), nil
			default: return math.NaN(), nil
		}
	}},
	"int": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		return int(math.Trunc(ParseFloat(a[0]))), nil
	}},

	"pi": {0, func (s *Scope, a ...interface{}) (interface{}, error) {
		return math.Pi, nil
	}},
	"sqrt": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		return math.Sqrt(ParseFloat(a[0])), nil
	}},
	"sin": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		return math.Sin(ParseFloat(a[0])), nil
	}},
	"cos": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		return math.Cos(ParseFloat(a[0])), nil
	}},
	"rad": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		return ParseFloat(a[0]) * (math.Pi / 180), nil
	}},
	"deg": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		return ParseFloat(a[0]) * (180 / math.Pi), nil
	}},
	"hypot": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		return math.Hypot(ParseFloat(a[0]), ParseFloat(a[1])), nil
	}},
	
	"min": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		if List(a).Less(0, 1) {
			return a[0], nil
		} else {
			return a[1], nil
		}
	}},
	"max": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		if List(a).Less(0, 1) {
			return a[1], nil
		} else {
			return a[0], nil
		}
	}},

	"lt": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		return List(a).Less(0, 1), nil
	}},
	"lte": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		return List(a).Less(0, 1) || List(a).Equal(0, 1), nil
	}},
	"eq": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		return List(a).Equal(0, 1), nil
	}},
	"neq": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		return !List(a).Equal(0, 1), nil
	}},
	"gt": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		return !(List(a).Less(0, 1) || List(a).Equal(0, 1)), nil
	}},
	"gte": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		return !List(a).Less(0, 1), nil
	}},

	"and": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		return ToBool(a[0]) && ToBool(a[1]), nil
	}},
	"or": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		return ToBool(a[0]) || ToBool(a[1]), nil
	}},
	"not": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		return !ToBool(a[0]), nil
	}},
	
	"first": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		return First(a[0])
	}},
	"last": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		return Last(a[0])
	}},
	"butfirst": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		return ButFirst(a[0])
	}},
	"butlast": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		return ButLast(a[0])
	}},
	"count": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		switch seq := a[0].(type) {
			case List: return len(seq), nil
			case []string: return len(seq), nil
			case string: return len(seq), nil
			default: return nil, Error{
				"Count expects a sequence, got: " +
				fmt.Sprint(a[0])}
		}
	}},
	
	"list": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		return List{a[0], a[1]}, nil
	}},

	"lowercase": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		return strings.ToLower(ToString(a[0])), nil
	}},
	"uppercase": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		return strings.ToUpper(ToString(a[0])), nil
	}},
	"trim": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		return strings.TrimSpace(ToString(a[0])), nil
	}},
	"ltrim": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		return strings.TrimLeft(ToString(a[0]), " \t\r\n\v"), nil
	}},
	"rtrim": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		return strings.TrimRight(ToString(a[0]), " \t\r\n\v"), nil
	}},

	"empty": {0, func (s *Scope, a ...interface{}) (interface{}, error) {
		return "", nil
	}},
	"space": {0, func (s *Scope, a ...interface{}) (interface{}, error) {
		return " ", nil
	}},
	"tab": {0, func (s *Scope, a ...interface{}) (interface{}, error) {
		return "\t", nil
	}},
	"nl": {0, func (s *Scope, a ...interface{}) (interface{}, error) {
		return "\n", nil
	}},
	
	"split": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		return splitre.Split(
			strings.TrimSpace(ToString(a[0])), -1), nil
	}},
	"word": {2,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		return ToString(a[0]) + ToString(a[1]), nil
	}},

	"starts-with": {2,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		return strings.HasPrefix(
			ToString(a[1]), ToString(a[0])), nil
	}},
	"ends-with": {2,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		return strings.HasSuffix(
			ToString(a[1]), ToString(a[0])), nil
	}},
	
	"to-string": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		return ToString(a[0]), nil
	}},
	"parse-int": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		return ParseInt(a[0]), nil
	}},
	"parse-float": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		return ParseFloat(a[0]), nil
	}},

	"is-string": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		_, ok := a[0].(string)
		return ok, nil
	}},
	"is-bool": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		_, ok := a[0].(bool)
		return ok, nil
	}},
	"is-int": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		_, ok := a[0].(int)
		return ok, nil
	}},
	"is-float": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		_, ok := a[0].(float64)
		return ok, nil
	}},
	"is-list": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		switch a[0].(type) {
			case List: return true, nil
			case []string: return true, nil
			default: return false, nil
		}
	}},
	"is-dict": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		_, ok := a[0].(Dict)
		return ok, nil
	}},
	"is-fn": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		_, ok := a[0].(Closure)
		return ok, nil
	}},
	"is-proc": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		_, ok := a[0].(Builtin)
		return ok, nil
	}},

	"is-space": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		return spacere.MatchString(ToString(a[0])), nil
	}},
	"is-alpha": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		return alphare.MatchString(ToString(a[0])), nil
	}},
	"is-alnum": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		return alnumre.MatchString(ToString(a[0])), nil
	}},
	"is-digit": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		return digitre.MatchString(ToString(a[0])), nil
	}},

	"rnd": {0, func (s *Scope, a ...interface{}) (interface{}, error) {
		return rand.Float64(), nil
	}},
	"random": {2, func (s *Scope, a ...interface{}) (interface{}, error) {
		low := ParseInt(a[0])
		high := ParseInt(a[1])
		return rand.Intn(high - low + 1) + low, nil
	}},
	"rerandom": {1,
	func (s *Scope, a ...interface{}) (interface{}, error) {
		rand.Seed(int64(ParseFloat(a[0])))
		return nil, nil
	}},
	"pick": {1, func (s *Scope, a ...interface{}) (interface{}, error) {
		return Pick(a[0])
	}},

	"timer": {0, func (s *Scope, a ...interface{}) (interface{}, error) {
		return float64(
			time.Now().UnixNano()) / (1000 * 1000 * 1000), nil
	}},
}

func init() {
	tmp := func (s *Scope, a ...interface{}) (interface{}, error) {
		if filename, ok := a[0].(string); ok {
			return Load(filename, Procedures, s)
		} else {
			return nil, Error{
				"Filename should be string in load, found: " +
				fmt.Sprint(a[0])}
		}
	}
	Procedures["load"] = Builtin{1, tmp}

	tmp = func (s *Scope, a ...interface{}) (interface{}, error) {
		if words, ok := a[0].([]string); ok {
			return Parse(words, Procedures)
		} else {
			return nil, Error{
				"Parse expects a list of strings, found: " +
				fmt.Sprint(a[0])}
		}
	}
	Procedures["parse"] = Builtin{1, tmp}
}

func main() {
	if len(os.Args) > 1 {
		toplevel := Scope{Names: Dict{}}
		code, err := Parse(os.Args[1:], Procedures)
		if err == nil {
			results, err2 := Results(code, &toplevel)
			if err2 == nil {
				for _, i := range(results) {
					if i != nil {
						fmt.Println(i)
					}
				}
			} else {
				fmt.Fprintln(Errs, err2)
			}
		} else {
			fmt.Fprintln(Errs, err)
		} 
	} else {
		fmt.Println("Lunar Logo alpha release, 2017-01-29")
		fmt.Printf("Compiled with %d procedures.\n", len(Procedures))

		fmt.Println("Usage:\n\tlunar.py [logo code...]")
		fmt.Println("\tlunar.py load <filename>")
	}
}
