package vm

import (
	"bytes"
	"fmt"
	"math"

	"chain/protocol/bc"
)

func opCheckOutput(vm *virtualMachine) error {
	if vm.tx == nil {
		return ErrContext
	}

	err := vm.applyCost(16)
	if err != nil {
		return err
	}

	code, err := vm.pop(true)
	if err != nil {
		return err
	}
	vmVersion, err := vm.popInt64(true)
	if err != nil {
		return err
	}
	if vmVersion < 0 {
		return ErrBadValue
	}
	assetID, err := vm.pop(true)
	if err != nil {
		return err
	}
	amount, err := vm.popInt64(true)
	if err != nil {
		return err
	}
	if amount < 0 {
		return ErrBadValue
	}
	refdatahash, err := vm.pop(true)
	if err != nil {
		return err
	}
	outputID, err := vm.pop(true)
	if err != nil {
		return err
	}
	// xxx check len(outputID)

	// xxx how to handle retirements? they don't have code to check

	var o *bc.Output
	for _, resultRef := range vm.tx.Outputs {
		id := resultRef.Hash()
		if bytes.Equal(outputID, id[:]) {
			o = resultRef.Entry.(*bc.Output)
			break
		}
	}
	if o == nil {
		return ErrBadValue
	}

	if o.Amount() != uint64(amount) {
		return vm.pushBool(false, true)
	}
	prog := o.ControlProgram()
	if prog.VMVersion != uint64(vmVersion) {
		return vm.pushBool(false, true)
	}
	if !bytes.Equal(prog.Code, code) {
		return vm.pushBool(false, true)
	}
	oAssetID := o.AssetID()
	if !bytes.Equal(oAssetID[:], assetID) {
		return vm.pushBool(false, true)
	}
	if len(refdatahash) > 0 {
		oRefDataHash := o.Data()
		if !bytes.Equal(oRefDataHash[:], refdatahash) {
			return vm.pushBool(false, true)
		}
	}
	return vm.pushBool(true, true)
}

func opAsset(vm *virtualMachine) error {
	if vm.tx == nil {
		return ErrContext
	}

	err := vm.applyCost(1)
	if err != nil {
		return err
	}

	var assetID bc.AssetID

	switch e := vm.input.Entry.(type) {
	case *bc.Spend:
		oEntry := e.SpentOutput().Entry
		if oEntry == nil {
			// xxx error
		}
		o, ok := oEntry.(*bc.Output)
		if !ok {
			// xxx error
		}
		assetID = o.AssetID()

	case *bc.Issuance:
		assetID = e.AssetID()

	default:
		// xxx error
	}

	return vm.push(assetID[:], true)
}

func opAmount(vm *virtualMachine) error {
	if vm.tx == nil {
		return ErrContext
	}

	err := vm.applyCost(1)
	if err != nil {
		return err
	}

	var amount uint64

	switch e := vm.input.Entry.(type) {
	case *bc.Spend:
		oEntry := e.SpentOutput().Entry
		if oEntry == nil {
			// xxx error
		}
		o, ok := oEntry.(*bc.Output)
		if !ok {
			// xxx error
		}
		amount = o.Amount()

	case *bc.Issuance:
		amount = e.Amount()

	default:
		// xxx error
	}

	return vm.pushInt64(int64(amount), true)
}

func opProgram(vm *virtualMachine) error {
	if vm.tx == nil {
		return ErrContext
	}

	err := vm.applyCost(1)
	if err != nil {
		return err
	}

	return vm.push(vm.mainprog, true)
}

func opMinTime(vm *virtualMachine) error {
	if vm.tx == nil {
		return ErrContext
	}

	err := vm.applyCost(1)
	if err != nil {
		return err
	}

	return vm.pushInt64(int64(vm.tx.MinTimeMS()), true)
}

func opMaxTime(vm *virtualMachine) error {
	if vm.tx == nil {
		return ErrContext
	}

	err := vm.applyCost(1)
	if err != nil {
		return err
	}

	maxTime := vm.tx.MaxTimeMS()
	if maxTime == 0 || maxTime > math.MaxInt64 {
		maxTime = uint64(math.MaxInt64)
	}

	return vm.pushInt64(int64(maxTime), true)
}

func opRefDataHash(vm *virtualMachine) error {
	if vm.tx == nil {
		return ErrContext
	}

	err := vm.applyCost(1)
	if err != nil {
		return err
	}

	var h bc.Hash

	switch e := vm.input.Entry.(type) {
	case *bc.Spend:
		h = e.Data()
	case *bc.Issuance:
		h = e.Data()
	default:
		// xxx error
	}

	return vm.push(h[:], true)
}

func opTxRefDataHash(vm *virtualMachine) error {
	if vm.tx == nil {
		return ErrContext
	}

	err := vm.applyCost(1)
	if err != nil {
		return err
	}

	h := vm.tx.Data()
	return vm.push(h[:], true)
}

func opOutputID(vm *virtualMachine) error {
	if vm.tx == nil {
		return ErrContext
	}

	sp, ok := vm.input.Entry.(*bc.Spend)
	if !ok {
		return ErrContext
	}
	if sp == nil {
		// xxx error
	}
	spent := sp.SpentOutput()
	outID := spent.Hash()

	err := vm.applyCost(1)
	if err != nil {
		return err
	}

	return vm.push(outID[:], true)
}

func opNonce(vm *virtualMachine) error {
	if vm.tx == nil {
		return ErrContext
	}

	_, ok := vm.input.Entry.(*bc.Issuance)
	if !ok {
		return ErrContext
	}

	err := vm.applyCost(1)
	if err != nil {
		return err
	}

	var nonce []byte
	// xxx
	return vm.push(nonce, true)
}

func opNextProgram(vm *virtualMachine) error {
	if vm.block == nil {
		return ErrContext
	}
	err := vm.applyCost(1)
	if err != nil {
		return err
	}
	return vm.push(vm.block.ConsensusProgram, true)
}

func opBlockTime(vm *virtualMachine) error {
	if vm.block == nil {
		return ErrContext
	}
	err := vm.applyCost(1)
	if err != nil {
		return err
	}
	if vm.block.TimestampMS > math.MaxInt64 {
		return fmt.Errorf("block timestamp out of range")
	}
	return vm.pushInt64(int64(vm.block.TimestampMS), true)
}
