package main

// contractNonConstMethodsTemplateContent contains the template string from contract_non_const_methods.go.tmpl
var contractNonConstMethodsTemplateContent = `{{- $contract := . -}}
{{- $logger := (print $contract.ShortVar "Logger") -}}
{{- range $i, $method := .NonConstMethods }}

// Transaction submission.
func ({{$contract.ShortVar}} *{{$contract.Class}}) {{$method.CapsName}}(
	{{$method.ParamDeclarations -}}
	{{- if $method.Payable -}}
	value *big.Int,
	{{ end }}
	transactionOptions ...chainutil.TransactionOptions,
) (*types.Transaction, error) {
	{{$logger}}.Debug(
		"submitting transaction {{$method.LowerName}}",
		{{if $method.Params -}}
		" params: ",
		fmt.Sprint(
			{{$method.Params}}
		),
		{{end -}}
		{{if $method.Payable -}}
		" value: ", value,
		{{- end}}
	)

	{{$contract.ShortVar}}.transactionMutex.Lock()
	defer {{$contract.ShortVar}}.transactionMutex.Unlock()

	// create a copy
	transactorOptions := new(bind.TransactOpts)
	*transactorOptions = *{{$contract.ShortVar}}.transactorOptions

	{{if $method.Payable -}}
	transactorOptions.Value = value
	{{- end }}

	if len(transactionOptions) > 1 {
		return nil, fmt.Errorf(
			"could not process multiple transaction options sets",
		)
	} else if len(transactionOptions) > 0 {
		transactionOptions[0].Apply(transactorOptions)
	}

	nonce, err := {{$contract.ShortVar}}.nonceManager.CurrentNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
	}

	transactorOptions.Nonce = new(big.Int).SetUint64(nonce)

	transaction, err := {{$contract.ShortVar}}.contract.{{$method.CapsName}}(
		transactorOptions,
		{{$method.Params}}
	)
	if err != nil {
		return transaction, {{$contract.ShortVar}}.errorResolver.ResolveError(
			err,
			{{$contract.ShortVar}}.transactorOptions.From,
			{{if $method.Payable -}}
			value
			{{- else -}}
			nil
			{{- end -}},
			"{{$method.LowerName}}",
			{{$method.Params}}
		)
	}

	{{$logger}}.Infof(
		"submitted transaction {{$method.LowerName}} with id: [%s] and nonce [%v]",
		transaction.Hash(),
		transaction.Nonce(),
	)

	go {{$contract.ShortVar}}.miningWaiter.ForceMining(
		transaction,
		transactorOptions,
		func(newTransactorOptions *bind.TransactOpts) (*types.Transaction, error) {
			// If original transactor options has a non-zero gas limit, that
			// means the client code set it on their own. In that case, we
			// should rewrite the gas limit from the original transaction
			// for each resubmission. If the gas limit is not set by the client
			// code, let the the submitter re-estimate the gas limit on each
			// resubmission.
			if transactorOptions.GasLimit != 0 {
				newTransactorOptions.GasLimit = transactorOptions.GasLimit
			}

			transaction, err := {{$contract.ShortVar}}.contract.{{$method.CapsName}}(
				newTransactorOptions,
				{{$method.Params}}
			)
			if err != nil {
				return nil, {{$contract.ShortVar}}.errorResolver.ResolveError(
					err,
					{{$contract.ShortVar}}.transactorOptions.From,
					{{if $method.Payable -}}
					value
					{{- else -}}
					nil
					{{- end -}},
					"{{$method.LowerName}}",
					{{$method.Params}}
				)
			}

			{{$logger}}.Infof(
				"submitted transaction {{$method.LowerName}} with id: [%s] and nonce [%v]",
				transaction.Hash(),
				transaction.Nonce(),
			)

			return transaction, nil
		},
	)

	{{$contract.ShortVar}}.nonceManager.IncrementNonce()

	return transaction, err
}

{{- $returnVar := print "result, " -}}
{{ if eq $method.Return.Type "" -}}
{{- $returnVar = "" -}}
{{- end }}

// Non-mutating call, not a transaction submission.
func ({{$contract.ShortVar}} *{{$contract.Class}}) Call{{$method.CapsName}}(
	{{$method.ParamDeclarations -}}
	{{- if $method.Payable -}}
	value *big.Int,
	{{ end -}}
	blockNumber *big.Int,
) ({{- if gt (len $method.Return.Type) 0 -}} {{$method.Return.Type}}, {{- end -}} error) {
	{{- if gt (len $method.Return.Type) 0 }}
	var result {{$method.Return.Type}}
	{{- else }}
	var result interface{} = nil
	{{- end }}

	err := chainutil.CallAtBlock(
		{{$contract.ShortVar}}.transactorOptions.From,
		blockNumber,
		{{- if $method.Payable -}}
		value,
		{{ else -}}
		nil,
		{{ end -}}
		{{$contract.ShortVar}}.contractABI,
		{{$contract.ShortVar}}.caller,
		{{$contract.ShortVar}}.errorResolver,
		{{$contract.ShortVar}}.contractAddress,
		"{{$method.LowerName}}",
		&result,
		{{$method.Params}}
	)

	return {{$returnVar}}err
}

func ({{$contract.ShortVar}} *{{$contract.Class}}) {{$method.CapsName}}GasEstimate(
	{{$method.ParamDeclarations -}}
) (uint64, error) {
	var result uint64

	result, err := chainutil.EstimateGas(
		{{$contract.ShortVar}}.callerOptions.From,
		{{$contract.ShortVar}}.contractAddress,
		"{{$method.LowerName}}",
		{{$contract.ShortVar}}.contractABI,
		{{$contract.ShortVar}}.transactor,
		{{$method.Params}}
	)

	return result, err
}

{{- end -}}
`
