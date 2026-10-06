package main

// contractConstMethodsTemplateContent contains the template string from contract_const_methods.go.tmpl
var contractConstMethodsTemplateContent = `{{- $contract := . -}}
{{- range $i, $method := .ConstMethods }}

{{- if $method.Return.Multi }}
type {{$method.Return.Type}} struct {
		{{$method.Return.Declarations}}
}
{{- end }}

func ({{$contract.ShortVar}} *{{$contract.Class}}) {{$method.CapsName}}(
	{{$method.ParamDeclarations -}}
	{{if $method.Payable -}} value *big.Int, {{- end -}}
) ({{$method.Return.Type}}, error) {
	{{- if and $method.Return.Multi (not $method.Return.Structured) }}
	{{$method.Return.Vars}}
	{{- else }}
	result,
	{{- end }} err := {{$contract.ShortVar}}.contract.{{$method.CapsName}}(
		{{$contract.ShortVar}}.callerOptions,
		{{$method.Params}}
	)

	{{ if and $method.Return.Multi (not $method.Return.Structured) }}
	result := {{$method.Return.Type -}}
	{
		{{- $method.Return.Vars -}}
	}
	{{- end }}

	if err != nil {
		return result, {{$contract.ShortVar}}.errorResolver.ResolveError(
			err,
			{{$contract.ShortVar}}.callerOptions.From,
			nil,
			"{{$method.LowerName}}",
			{{$method.Params}}
		)
	}

	return result, err
}

func ({{$contract.ShortVar}} *{{$contract.Class}}) {{$method.CapsName}}AtBlock(
	{{$method.ParamDeclarations -}}
	{{if $method.Payable -}} value *big.Int, {{- end -}}
	blockNumber *big.Int,
) ({{$method.Return.Type}}, error) {
	var result {{$method.Return.Type}}

	err := chainutil.CallAtBlock(
		{{$contract.ShortVar}}.callerOptions.From,
		blockNumber,
		nil,
		{{$contract.ShortVar}}.contractABI,
		{{$contract.ShortVar}}.caller,
		{{$contract.ShortVar}}.errorResolver,
		{{$contract.ShortVar}}.contractAddress,
		"{{$method.LowerName}}",
		&result,
		{{$method.Params}}
	)

	return result, err
}

{{end -}}
`
