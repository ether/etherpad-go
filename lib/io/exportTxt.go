package io

import (
	pad2 "github.com/ether/etherpad-go/lib/models/pad"
	pad3 "github.com/ether/etherpad-go/lib/pad"
)

type ExportTxt struct {
	PadManager *pad3.Manager
}

func (e *ExportTxt) GetPadTxtDocument(padId string, revNum *int) (*string, error) {
	foundPad, err := e.PadManager.GetPad(padId, nil, nil)
	if err != nil {
		return nil, err
	}
	return GetPadTxt(*foundPad, revNum)
}

func GetPadTxt(pad pad2.Pad, revNum *int) (*string, error) {
	atext := pad.AText

	if revNum != nil {
		optAtext := pad.GetInternalRevisionAText(*revNum)
		if optAtext != nil {
			atext = *optAtext
		}
	}
	return pad3.GetTxtFromAText(&pad, atext)
}
