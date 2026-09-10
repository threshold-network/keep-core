package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

// The extracted name + payability of methods from ABI JSON.
type methodPayableInfo struct {
	Name    string
	Payable bool
}

var (
	classNameRegexp *regexp.Regexp
	shortVarRegexp  *regexp.Regexp
)

// bindBasicTypeGo converts basic solidity types (except array, slice and tuple)
// to Go ones.
//
// This and the three functions below (bindStructTypeGo, bindTopicTypeGo,
// structured) are ported from go-ethereum's accounts/abi/bind package (bind.go,
// as of go-ethereum v1.13.15). This file used to reach that package's
// unexported bindStructTypeGo/bindTopicTypeGo/structured directly via
// //go:linkname, but go-ethereum reworked accounts/abi/bind and those symbols
// no longer exist under any name as of v1.17.3, so their logic is ported and
// kept locally instead.
func bindBasicTypeGo(kind abi.Type) string {
	switch kind.T {
	case abi.AddressTy:
		return "common.Address"
	case abi.IntTy, abi.UintTy:
		parts := regexp.MustCompile(`(u)?int([0-9]*)`).FindStringSubmatch(kind.String())
		switch parts[2] {
		case "8", "16", "32", "64":
			return fmt.Sprintf("%sint%s", parts[1], parts[2])
		}
		return "*big.Int"
	case abi.FixedBytesTy:
		return fmt.Sprintf("[%d]byte", kind.Size)
	case abi.BytesTy:
		return "[]byte"
	case abi.FunctionTy:
		return "[24]byte"
	default:
		// string, bool types
		return kind.String()
	}
}

// bindStructTypeGo converts a Solidity type to a Go one, assigning and caching
// a name for each tuple (struct) type the first time it's encountered so that
// repeated references to the same struct type resolve to the same name. Nested
// tuple fields are registered before the enclosing tuple's own fallback name is
// computed, matching go-ethereum's original registration order.
func bindStructTypeGo(kind abi.Type, structs map[string]string) string {
	switch kind.T {
	case abi.TupleTy:
		// We compose a raw struct name and a canonical parameter expression
		// together here. The reason is before solidity v0.5.11, kind.TupleRawName
		// is empty, so we use canonical parameter expression to distinguish
		// different struct definitions. From the consideration of backward
		// compatibility, we concat these two together so that if kind.TupleRawName
		// is not empty, it can have a unique id.
		id := kind.TupleRawName + kind.String()
		if name, exist := structs[id]; exist {
			return name
		}
		for _, elem := range kind.TupleElems {
			bindStructTypeGo(*elem, structs)
		}
		name := kind.TupleRawName
		if name == "" {
			name = fmt.Sprintf("Struct%d", len(structs))
		}
		name = abi.ToCamelCase(name)
		structs[id] = name
		return name
	case abi.ArrayTy:
		return fmt.Sprintf("[%d]", kind.Size) + bindStructTypeGo(*kind.Elem, structs)
	case abi.SliceTy:
		return "[]" + bindStructTypeGo(*kind.Elem, structs)
	default:
		return bindBasicTypeGo(kind)
	}
}

// bindTopicTypeGo converts a Solidity topic type to a Go one. It is almost the
// same functionality as for simple types, but dynamic types get converted to
// hashes.
func bindTopicTypeGo(kind abi.Type, structs map[string]string) string {
	bound := bindStructTypeGo(kind, structs)
	if bound == "string" || bound == "[]byte" {
		bound = "common.Hash"
	}
	return bound
}

// structured checks whether a list of ABI data types has enough information to
// operate through a proper Go struct, or if flat returns are needed.
func structured(args abi.Arguments) bool {
	if len(args) < 2 {
		return false
	}
	exists := make(map[string]bool)
	for _, out := range args {
		// If the name is anonymous, we can't organize into a struct.
		if out.Name == "" {
			return false
		}
		// If the field name is empty when normalized or collides
		// (var, Var, _var, _Var), we can't organize into a struct.
		field := abi.ToCamelCase(out.Name)
		if field == "" || exists[field] {
			return false
		}
		exists[field] = true
	}
	return true
}

func init() {
	var err error
	classNameRegexp, err = regexp.Compile("ImplV.*")
	if err != nil {
		panic(fmt.Sprintf(
			"Failed to compile class name regular expression: [%v].",
			"ImplV.*",
		))
	}

	shortVarRegexp, err = regexp.Compile("([A-Z])[^A-Z]*")
	if err != nil {
		panic(fmt.Sprintf(
			"Failed to compile class name regular expression: [%v].",
			"([A-Z])[^A-Z]*",
		))
	}
}

// The following structs are sent into the templates for compilation.
type contractInfo struct {
	HostChainModule  string
	ChainUtilPackage string
	// GenPackage is the import path of the gen/ directory this contract's
	// bindings are being generated into (e.g.
	// ".../pkg/chain/ethereum/tbtc/gen"). It's used to explicitly import the
	// sibling abi (and, for commands, contract) packages rather than relying
	// on goimports to guess them: this module has several gen/ directories
	// that each declare a like-named abi/contract package, and goimports
	// cannot reliably disambiguate between them. Empty GenPackage skips the
	// explicit import, falling back to goimports auto-resolution.
	GenPackage      string
	Class           string
	AbiClass        string
	FullVar         string
	ShortVar        string
	DashedName      string
	ConstMethods    []methodInfo
	NonConstMethods []methodInfo
	Events          []eventInfo
}

type cmdArgInfo struct {
	Name       string
	Type       string
	GoType     string
	ParsingFn  string
	Structured bool
}

type methodInfo struct {
	CapsName          string
	LowerName         string
	DashedName        string
	Modifiers         string
	Payable           bool
	CommandCallable   bool
	Params            string
	ParamDeclarations string
	CmdArgInfos       []cmdArgInfo
	Return            returnInfo
}

type returnInfo struct {
	Multi bool
	// Methods can return multiple outputs. If all the outputs are named they
	// are combined into a struct.
	Structured   bool
	Type         string
	Declarations string
	Vars         string
}

type eventInfo struct {
	CapsName                  string
	LowerName                 string
	SubscriptionCapsName      string
	ShortVar                  string
	SubscriptionShortVar      string
	IndexedFilters            string
	ParamExtractors           string
	ParamDeclarations         string
	IndexedFilterExtractors   string
	IndexedFilterDeclarations string
	IndexedFilterFields       string
}

func buildContractInfo(
	hostChainModule string,
	chainUtilPackage string,
	genPackage string,
	abiClassName string,
	abi *abi.ABI,
	payableInfo []methodPayableInfo,
) contractInfo {
	payableMethods := make(map[string]struct{})
	for _, methodPayableInfo := range payableInfo {
		if methodPayableInfo.Payable {
			normalizedName := camelCase(methodPayableInfo.Name)
			_, ok := payableMethods[normalizedName]
			for idx := 0; ok; idx++ {
				normalizedName = fmt.Sprintf("%s%d", normalizedName, idx)
				_, ok = payableMethods[normalizedName]
			}
			payableMethods[normalizedName] = struct{}{}
		}
	}

	goClassName := classNameRegexp.ReplaceAll([]byte(abiClassName), nil)
	shortVar := strings.ToLower(string(shortVarRegexp.ReplaceAll(
		[]byte(goClassName),
		[]byte("$1"),
	)))
	dashedName := strings.ToLower(string(shortVarRegexp.ReplaceAll(
		[]byte(lowercaseFirst(string(goClassName))),
		[]byte("-$0"),
	)))

	structs := make(map[string]string)
	constMethods, nonConstMethods := buildMethodInfo(payableMethods, abi.Methods, structs)
	events := buildEventInfo(shortVar, abi.Events, structs)

	return contractInfo{
		hostChainModule,
		chainUtilPackage,
		genPackage,
		string(goClassName),
		abiClassName,
		lowercaseFirst(string(goClassName)),
		string(shortVar),
		string(dashedName),
		constMethods,
		nonConstMethods,
		events,
	}
}

func buildMethodInfo(
	payableMethods map[string]struct{},
	methodsByName map[string]abi.Method,
	structs map[string]string,
) (constMethods []methodInfo, nonConstMethods []methodInfo) {
	nonConstMethods = make([]methodInfo, 0, len(methodsByName))
	constMethods = make([]methodInfo, 0, len(methodsByName))

	for name, method := range methodsByName {
		normalizedName := camelCase(name)
		dashedName := strings.ToLower(string(shortVarRegexp.ReplaceAll(
			[]byte(normalizedName),
			[]byte("-$0"),
		)))

		_, payable := payableMethods[normalizedName]
		commandCallable := true

		modifiers := make([]string, 0, 0)

		if method.StateMutability != "" {
			modifiers = append(modifiers, method.StateMutability)
		}

		// Legacy indicators generated by compiler before v0.6.0
		if payable {
			modifiers = append(modifiers, "payable")
		}
		if method.Constant {
			modifiers = append(modifiers, "constant")
		}

		modifierString := strings.Join(modifiers, " ")
		if len(modifiers) > 0 {
			modifierString += " "
		}

		paramDeclarations := ""
		params := ""
		cmdArgInfos := make([]cmdArgInfo, 0, 0)

		for index, param := range method.Inputs {
			goType := bindType(param.Type, structs)

			var paramName string
			if param.Name == "" {
				paramName = fmt.Sprintf("arg%d", index)
			} else {
				paramName = fmt.Sprintf("arg_%v", param.Name)
			}

			paramDeclarations += fmt.Sprintf("%v %v,\n", paramName, goType)
			params += fmt.Sprintf("%v,\n", paramName)

			// Build cmdArgInfos used for CLI code generator
			cmdParamName := paramName
			cmdParamStructured := param.Type.TupleType != nil
			cmdParsingFn := ""

			if cmdParamStructured {
				cmdParamName += "_json"
			} else {
			goTypeSwitch:
				switch goType {
				case "[]byte":
					cmdParsingFn = "hexutil.Decode(%s)"
				case "[20]byte":
					cmdParsingFn = "decode.ParseBytes20(%s)"
				case "[32]byte":
					cmdParsingFn = "decode.ParseBytes32(%s)"
				case "common.Address":
					cmdParsingFn = "chainutil.AddressFromHex(%s)"
				case "*big.Int":
					cmdParsingFn = "hexutil.DecodeBig(%s)"
				case "bool":
					cmdParsingFn = "strconv.ParseBool(%s)"
				default:
					intParts := regexp.MustCompile(`^(u|)int([0-9]*)$`).FindStringSubmatch(goType)
					if len(intParts) > 0 {
						switch intParts[2] {
						case "8", "16", "32", "64":
							var template string
							if intParts[1] == "u" {
								template = "decode.ParseUint[uint%s](%%s, %s)"
							} else {
								template = "decode.ParseInt[int%s](%%s, %s)"
							}

							cmdParsingFn = fmt.Sprintf(template, intParts[2], intParts[2])
							break goTypeSwitch
						}
					}

					// TODO: Add support for more types, i.a. slices, arrays.
					fmt.Printf(
						"WARNING: Unsupported param type for method %s:\n"+
							"  ABI Type: %s\n"+
							"  Go Type:  %s\n"+
							"  the method won't be callable with 'ethereum' command\n",
						name,
						param.Type,
						goType,
					)
					commandCallable = false
				}
			}

			cmdArgInfos = append(
				cmdArgInfos,
				cmdArgInfo{
					Name:       cmdParamName,
					Type:       param.Type.String(),
					GoType:     goType,
					ParsingFn:  cmdParsingFn,
					Structured: cmdParamStructured,
				})
		}

		returned := returnInfo{}
		if len(method.Outputs) > 1 {
			returned.Multi = true
			returned.Type = strings.Replace(normalizedName, "get", "", 1)

			for index, output := range method.Outputs {
				goType := bindType(output.Type, structs)

				returned.Declarations += fmt.Sprintf(
					"\t%v %v\n",
					uppercaseFirst(output.Name),
					goType,
				)

				returned.Structured = structured(method.Outputs)

				// For structured outputs return one variable.
				if returned.Structured {
					returned.Vars = "ret,"
					continue
				}

				var varName string
				if output.Name == "" {
					varName = fmt.Sprintf("ret%d", index)
				} else {
					varName = fmt.Sprintf("ret_%v", output.Name)
				}

				returned.Vars += fmt.Sprintf("%v,", varName)
			}
		} else if len(method.Outputs) == 0 {
			returned.Multi = false
		} else {
			returned.Multi = false
			returned.Type = bindType(method.Outputs[0].Type, structs)
			returned.Vars += "ret,"
		}

		info := methodInfo{
			uppercaseFirst(normalizedName),
			lowercaseFirst(normalizedName),
			dashedName,
			modifierString,
			payable,
			commandCallable,
			params,
			paramDeclarations,
			cmdArgInfos,
			returned,
		}

		if isMethodConstant(method) {
			constMethods = append(constMethods, info)
		} else {
			nonConstMethods = append(nonConstMethods, info)
		}
	}

	sort.Sort(methodInfoSlice(constMethods))
	sort.Sort(methodInfoSlice(nonConstMethods))

	return constMethods, nonConstMethods
}

func buildEventInfo(
	contractShortVar string,
	eventsByName map[string]abi.Event,
	structs map[string]string,
) []eventInfo {
	eventInfos := make([]eventInfo, 0, len(eventsByName))
	for name, event := range eventsByName {

		capsName := uppercaseFirst(name)
		lowerName := lowercaseFirst(name)
		subscriptionCapsName := uppercaseFirst(contractShortVar) +
			capsName +
			"Subscription"

		shortVar := strings.ToLower(string(shortVarRegexp.ReplaceAll(
			[]byte(name),
			[]byte("$1"),
		)))
		subscriptionShortVar := shortVar + "s"

		paramDeclarations := ""
		paramExtractors := ""
		indexedFilterExtractors := ""
		indexedFilterDeclarations := ""
		indexedFilterFields := ""
		indexedFilters := ""
		for _, param := range event.Inputs {
			upperParam := uppercaseFirst(param.Name)
			goType := bindType(param.Type, structs)

			paramExtractors += fmt.Sprintf("event.%v,\n", upperParam)

			if param.Indexed {
				// For event's indexed parameter abigen uses dedicated type binding
				// for topic.
				paramDeclarations += fmt.Sprintf("%v %v,\n", upperParam, bindTopicType(param.Type, structs))

				indexedFilterExtractors += fmt.Sprintf("%v.%vFilter,\n", subscriptionShortVar, param.Name)
				indexedFilterDeclarations += fmt.Sprintf("%vFilter []%v,\n", param.Name, goType)
				indexedFilterFields += fmt.Sprintf("%vFilter []%v\n", param.Name, goType)
				indexedFilters += fmt.Sprintf("%vFilter,\n", param.Name)
			} else {
				paramDeclarations += fmt.Sprintf("%v %v,\n", upperParam, goType)
			}
		}

		paramDeclarations += "blockNumber uint64,\n"
		paramExtractors += "event.Raw.BlockNumber,\n"

		eventInfos = append(eventInfos, eventInfo{
			capsName,
			lowerName,
			subscriptionCapsName,
			shortVar,
			subscriptionShortVar,
			indexedFilters,
			paramExtractors,
			paramDeclarations,
			indexedFilterExtractors,
			indexedFilterDeclarations,
			indexedFilterFields,
		})
	}

	sort.Sort(eventInfoSlice(eventInfos))

	return eventInfos
}

func uppercaseFirst(str string) string {
	if len(str) == 0 {
		return str
	}

	str = strings.TrimPrefix(str, "_")

	return strings.ToUpper(str[0:1]) + str[1:]
}

func lowercaseFirst(str string) string {
	if len(str) == 0 {
		return str
	}

	str = strings.TrimPrefix(str, "_")

	return strings.ToLower(str[0:1]) + str[1:]
}

func camelCase(input string) string {
	parts := strings.Split(input, "_")
	for i, s := range parts {
		if len(s) > 0 {
			parts[i] = strings.ToUpper(s[:1]) + s[1:]
		}
	}
	return lowercaseFirst(strings.Join(parts, ""))
}

// For sorting purposes, we define the following interfaces on methodInfo and
// eventInfo slices.
type methodInfoSlice []methodInfo

func (mis methodInfoSlice) Len() int {
	return len(mis)
}
func (mis methodInfoSlice) Less(i, j int) bool {
	return mis[i].LowerName < mis[j].LowerName
}
func (mis methodInfoSlice) Swap(i, j int) {
	mis[i], mis[j] = mis[j], mis[i]
}

type eventInfoSlice []eventInfo

func (eis eventInfoSlice) Len() int {
	return len(eis)
}
func (eis eventInfoSlice) Less(i, j int) bool {
	return eis[i].LowerName < eis[j].LowerName
}
func (eis eventInfoSlice) Swap(i, j int) {
	eis[i], eis[j] = eis[j], eis[i]
}

// Verifies if a method should be considered as constant based on the modifier.
// Constants methods are `view` or `pure`. For compatibility with code generated
// by compilers before v0.6.0 verify also a legacy `Constant` identifier.
func isMethodConstant(method abi.Method) bool {
	return method.StateMutability == "view" ||
		method.StateMutability == "pure" ||
		method.Constant
}

// Converts solidity type to a Go type.
func bindType(kind abi.Type, structs map[string]string) string {
	return resolveGoType(kind, bindStructTypeGo(kind, structs))
}

// Converts solidity topic type to a Go type.
func bindTopicType(kind abi.Type, structs map[string]string) string {
	return resolveGoType(kind, bindTopicTypeGo(kind, structs))
}

func resolveGoType(kind abi.Type, goType string) string {
	switch kind.T {
	case abi.TupleTy:
		// Bindings for structs are expected to be generated into the `abi` package
		// by the abigen command.
		goType = "abi." + goType
	case abi.SliceTy:
		// Look for a struct nested in a slice.
		if kind.Elem.T == abi.TupleTy {
			goType = strings.Replace(goType, "[]", "[]abi.", 1)
		}
	}

	return goType
}
