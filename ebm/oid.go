package ebm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
)

type OIDType uint32

const (
	TypeUint32  = 0
	TypeInt32   = 1
	TypeUint16  = 2
	TypeInt16   = 3
	TypeUint8   = 4
	TypeInt8    = 5
	TypeString  = 6
	TypeBool    = 7
	TypeInvalid = 8
)

type OIDAccessModes uint32

const (
	AccessModeRead      OIDAccessModes = 0
	AccessModeWrite     OIDAccessModes = 1
	AccessModeReadWrite OIDAccessModes = 2
)

var oidTypeDesc = map[OIDType]string{
	0: "uint32",
	1: "int32",
	2: "uint16",
	3: "int16",
	4: "uint8",
	5: "int8",
	6: "string",
	7: "bool",
	8: "invalid",
}

type OID struct {
	OID         [3]uint32
	Length      uint32
	Offset      uint32
	Type        OIDType
	AccessModes OIDAccessModes
}

type oidRequest struct {
	OID    [3]uint32
	Offset uint32
	Length uint32
	Type   uint32
}

func (o *OID) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, oidRequest{
		OID:    o.OID,
		Offset: o.Offset,
		Length: o.Length,
		Type:   uint32(o.Type),
	}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func ParseOID(d []byte) (any, error) {
	var req oidRequest
	if err := binary.Read(bytes.NewReader(d), binary.BigEndian, &req); err != nil {
		return nil, err
	}
	payload := d[binary.Size(&req):]
	switch OIDType(req.Type) {
	case TypeUint32:
		return binary.BigEndian.Uint32(payload[:4]), nil
	case TypeUint16:
		return binary.BigEndian.Uint32(payload[:4]), nil
	case TypeUint8:
		if req.Length == 1 {
			return uint8(payload[0]), nil
		} else {
			return payload[0:req.Length], nil
		}
	case TypeString:
		return strings.TrimRight(string(payload[0:req.Length]), "\x00 "), nil
	case TypeBool:
		return payload[0] == 1, nil
	default:
		return nil, fmt.Errorf("unknown type %v", oidTypeDesc[OIDType(req.Type)])
	}
}

func MarshalOID(o *OID, val any) ([]byte, error) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, oidRequest{
		OID:    o.OID,
		Offset: o.Offset,
		Length: o.Length,
		Type:   uint32(o.Type),
	}); err != nil {
		return nil, err
	}
	switch o.Type {
	case TypeUint32:
		binary.Write(&buf, binary.BigEndian, val.(uint32))
	case TypeUint16:
		binary.Write(&buf, binary.BigEndian, val.(uint16))
	case TypeUint8:
		switch x := val.(type) {
		case uint8:
			binary.Write(&buf, binary.BigEndian, x)
		case []uint8:
			binary.Write(&buf, binary.BigEndian, x)
		default:
			return nil, fmt.Errorf("invalid type %T for uint8 oid", val)
		}
	case TypeString:
		strval := val.(string)
		buf.WriteString(strval)
		for i := len(strval); i < int(o.Length); i++ {
			buf.WriteByte(0)
		}
	case TypeBool:
		if val.(bool) {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}
	default:
		return nil, fmt.Errorf("unknown type %v", oidTypeDesc[OIDType(o.Type)])
	}
	return buf.Bytes(), nil
}

// G.994.1 Vendor ID
type VendorID struct {
	CountryCode  uint16
	ProviderCode string
	VendorInfo   uint16
}

func ParseVendorID(vid string) VendorID {
	return VendorID{
		CountryCode:  binary.BigEndian.Uint16([]byte(vid[:2])),
		ProviderCode: vid[2:6],
		VendorInfo:   binary.BigEndian.Uint16([]byte(vid[6:])),
	}
}

func newOIDUint32(a, b, c uint32) OID {
	return OID{
		OID:    [3]uint32{a, b, c},
		Length: 1,
		Type:   TypeUint32,
	}
}
func newOIDUint16(a, b, c uint32) OID {
	return OID{
		OID:    [3]uint32{a, b, c},
		Length: 1,
		Type:   TypeUint16,
	}
}

func newOIDUint8(a, b, c uint32) OID {
	return OID{
		OID:    [3]uint32{a, b, c},
		Length: 1,
		Type:   TypeUint8,
	}
}

func newOIDString(a, b, c, len uint32) OID {
	return OID{
		OID:    [3]uint32{a, b, c},
		Length: len,
		Type:   TypeString,
	}
}

var OidTxPackets = newOIDUint32(11, 27, 21)
var OidTxBytes = newOIDUint32(11, 27, 20)
var OidRxErrors = newOIDUint32(11, 27, 5)
var OidRxPackets = newOIDUint32(11, 27, 1)
var OidRxBytes = newOIDUint32(11, 27, 0)

// OidTicks is used to detect a stuck modem by checking if it stops
// incrementing.
var OidTicks = newOIDUint32(11, 21, 0)

var OidLogControl = OID{
	OID:         [3]uint32{11, 17, 4},
	Length:      1,
	Type:        TypeUint32,
	AccessModes: AccessModeReadWrite,
}

var OidConsoleControl = OID{
	OID:         [3]uint32{11, 17, 3},
	Length:      1,
	Type:        TypeUint32,
	AccessModes: AccessModeReadWrite,
}

// Modem
var OidMeasuredTimeUpstream = newOIDUint32(11, 14, 44)
var OidMeasuredTimeDownstream = newOIDUint32(11, 14, 43)
var OidErrorFreeBitsUpstream = newOIDUint32(11, 14, 42)
var OidErrorFreeBitsDownstream = newOIDUint32(11, 14, 41)
var OidFarEndRetransmittedDTU = newOIDUint32(11, 14, 40)
var OidNearEndRetransmittedDTU = newOIDUint32(11, 14, 39)
var OidFarEndUncorrectedDTU = newOIDUint32(11, 14, 38)
var OidNearEndUncorrectedDTU = newOIDUint32(11, 14, 37)
var OidFarEndCodeViolations = newOIDUint32(11, 14, 36)
var OidNearEndCodeViolations = newOIDUint32(11, 14, 35)

var OidFailedFullInits = newOIDUint32(11, 14, 20)
var OidFullInits = newOIDUint32(11, 14, 19)
var OidFarEndUnavailableSeconds = newOIDUint32(11, 14, 18)
var OidNearEndUnavailableSeconds = newOIDUint32(11, 14, 17)
var OidFarEndLossOfRMCSeconds = newOIDUint32(11, 14, 16)
var OidNearEndLossOfRMCSeconds = newOIDUint32(11, 14, 15)
var OidFarEndLossOfSignalSeconds = newOIDUint32(11, 14, 14)
var OidNearEndLossOfSignalSeconds = newOIDUint32(11, 14, 13)
var OidFarEndSeverelyErroredSeconds = newOIDUint32(11, 14, 12)
var OidNearEndSeverelyErroredSeconds = newOIDUint32(11, 14, 11)
var OidFarEndErroredSeconds = newOIDUint32(11, 14, 10)
var OidNearEndErroredSeconds = newOIDUint32(11, 14, 9)
var OidFarEndLossOfPower = newOIDUint32(11, 14, 8)
var OidNearEndLossOfPower = newOIDUint32(11, 14, 7)
var OidFarEndLossOfMargin = newOIDUint32(11, 14, 6)
var OidNearEndLossOfMargin = newOIDUint32(11, 14, 5)
var OidFarEndLossOfRMC = newOIDUint32(11, 14, 4)
var OidNearEndLossOfRMC = newOIDUint32(11, 14, 3)
var OidFarEndLossOfSignal = newOIDUint32(11, 14, 2)
var OidNearEndLossOfSignal = newOIDUint32(11, 14, 1)

// 0 : IDLE
var OidModemStatus = newOIDUint8(11, 10, 1)
var OidCmdStatus = OID{
	OID:         [3]uint32{11, 10, 0},
	Length:      1,
	Type:        TypeBool,
	AccessModes: AccessModeReadWrite,
}
var OidRepeatCommand = OID{
	OID:         [3]uint32{11, 1, 2},
	Length:      1,
	Type:        TypeUint8,
	AccessModes: AccessModeWrite,
}

// SFP To IDLE : 0x51b0 0

var OidHostCommand = OID{
	OID:         [3]uint32{11, 1, 0},
	Length:      1,
	Type:        TypeUint8,
	AccessModes: AccessModeWrite,
}

// Identifying info
// Writable
var OidNetworkTerminationSerial = newOIDString(10, 12, 9, 32)
var OidNetworkTerminationVendor = newOIDString(10, 12, 7, 8)

var OidDistributionPointUnitVendor = newOIDString(10, 12, 6, 8)
var OidDistributionPointUnitSerial = newOIDString(10, 12, 8, 32)

var OidFTURSelftest = newOIDString(10, 12, 5, 4)
var OIDFTUOSelftest = newOIDString(10, 12, 4, 4)

var OidXDSLTerminationUnitRemoteVersion = newOIDString(10, 12, 3, 16)
var OidXDSLTerminationUnitCentralVersion = newOIDString(10, 12, 2, 16)
var OidXDSLTerminationUnitRemoteVendor = newOIDString(10, 12, 1, 8)
var OidXDSLTerminationUnitCentralVendor = newOIDString(10, 12, 0, 8)

var OID_FECDTU_US = newOIDUint8(10, 10, 21)
var OID_FECDTU_DS = newOIDUint8(10, 10, 20)
var OID_FECRED_US = newOIDUint8(10, 10, 19)
var OID_FECRED_DS = newOIDUint8(10, 10, 18)
var OID_FECLEN_US = newOIDUint8(10, 10, 17)
var OID_FECLEN_DS = newOIDUint8(10, 10, 16)

var OidAttainableNetDataRateUpstream = newOIDUint32(10, 10, 7)
var OidAttainableNetDataRateDownstream = newOIDUint32(10, 10, 6)

var OidExpectedThroughputRateUpstream = newOIDUint32(10, 10, 3)
var OidExpectedThroughputRateDownstream = newOIDUint32(10, 10, 2)

var OidNetDataRateUpstream = newOIDUint32(10, 10, 1)
var OidNetDataRateDownstream = newOIDUint32(10, 10, 0)

var OID_SNPRS_USb = OID{
	OID:    [3]uint32{10, 9, 17},
	Length: 1024,
	Offset: 1024,
	Type:   TypeUint8,
}

var OID_SNPRS_USa = OID{
	OID:    [3]uint32{10, 9, 17},
	Length: 1024,
	Type:   TypeUint8,
}

var OidSNRSubCarrierGroupSizeUpstream = newOIDUint8(10, 9, 16)

var OID_SNPRS_DSb = OID{
	OID:    [3]uint32{10, 9, 14},
	Length: 1024,
	Offset: 1024,
	Type:   TypeUint8,
}

var OID_SNPRS_DSa = OID{
	OID:    [3]uint32{10, 9, 14},
	Length: 1024,
	Type:   TypeUint8,
}

var OidSNRSubCarrierGroupSizeDownstream = newOIDUint8(10, 9, 13)

var OidPowerUpstream = newOIDUint16(10, 9, 9)
var OidPowerDownstream = newOIDUint16(10, 9, 8)

var OidSignalToNoiseRatioMarginUpstream = newOIDUint16(10, 9, 5)
var OidSignalToNoiseRatioMarginDownstream = newOIDUint16(10, 9, 4)
var OidMaxNetDataRateUpstream = newOIDUint16(10, 1, 1)
var OidMaxNetDataRateDownstream = newOIDUint16(10, 1, 0)
