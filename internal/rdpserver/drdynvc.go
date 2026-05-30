package rdpserver

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/rcarmo/go-rdp-android/internal/input"
)

const (
	drdynvcStaticChannelName = "drdynvc"

	drdynvcCmdCreate     uint8 = 0x01
	drdynvcCmdDataFirst  uint8 = 0x02
	drdynvcCmdData       uint8 = 0x03
	drdynvcCmdClose      uint8 = 0x04
	drdynvcCmdCapability uint8 = 0x05

	drdynvcCapsVersion1 uint16 = 0x0001

	drdynvcCreateOK            uint32 = 0x00000000
	drdynvcCreateAlreadyExists uint32 = 0x800700b7
	drdynvcCreateNoListener    uint32 = 0x80070490

	channelFlagFirst uint32 = 0x00000001
	channelFlagLast  uint32 = 0x00000002

	drdynvcMaxStaticPayload = 1024 * 1024
	drdynvcMaxPDUSize       = 1024 * 1024
	drdynvcMaxFragmentSize  = 1024 * 1024
	drdynvcWriteChunkSize   = 1200
	drdynvcMaxFragments     = 8
	drdynvcFragmentTTL      = 30 * time.Second
)

type drdynvcManager struct {
	staticChannelID        uint16
	capsReceived           bool
	negotiatedCapsVersion  uint16
	rdpeiChannelID         uint32
	hasRDPEIChannel        bool
	rdpgfxChannelID        uint32
	hasRDPGFXChannel       bool
	pendingRDPGFXChannel   bool
	rdpgfxCapsConfirmed    bool
	rdpgfxCapability       rdpgfxCapabilitySet
	nextServerDVCChannelID uint32
	channels               map[uint32]string
	fragments              map[uint32]*drdynvcFragment
	touchLifecycle         *input.TouchLifecycleCoalescer
	sink                   input.Sink
	metrics                serverMetrics
}

type drdynvcFragment struct {
	expected  uint32
	data      []byte
	updatedAt time.Time
}

type staticVirtualChannelPDU struct {
	Length uint32
	Flags  uint32
	Data   []byte
}

type drdynvcPDU struct {
	Header       drdynvcHeader
	ChannelID    uint32
	Name         string
	Data         []byte
	Length       uint32
	Version      uint16
	CreationCode uint32
}

type drdynvcHeader struct {
	CbChID uint8
	Sp     uint8
	Cmd    uint8
}

func newDRDYNVCManager(channels []clientChannel, sink input.Sink, metrics serverMetrics) *drdynvcManager {
	m := &drdynvcManager{channels: make(map[uint32]string), fragments: make(map[uint32]*drdynvcFragment), touchLifecycle: input.NewTouchLifecycleCoalescer(), sink: sink, metrics: metrics, nextServerDVCChannelID: 100}
	for _, ch := range channels {
		if strings.EqualFold(ch.Name, drdynvcStaticChannelName) {
			m.staticChannelID = ch.ID
			break
		}
	}
	return m
}

func (m *drdynvcManager) enabled() bool { return m != nil && m.staticChannelID != 0 }

func (m *drdynvcManager) handleStaticPDU(conn net.Conn, payload []byte) error {
	m.cleanupFragments(time.Now())
	staticPDU, err := parseStaticVirtualChannelPDU(payload)
	if err != nil {
		return err
	}
	if staticPDU.Flags&channelFlagFirst == 0 || staticPDU.Flags&channelFlagLast == 0 {
		return fmt.Errorf("fragmented static drdynvc channel PDU not yet supported flags=0x%08x", staticPDU.Flags)
	}
	pdu, err := parseDRDYNVCPDU(staticPDU.Data)
	if err != nil {
		return err
	}
	tracef("drdynvc_pdu", "cmd=%d channel=%d name=%q data_len=%d", pdu.Header.Cmd, pdu.ChannelID, pdu.Name, len(pdu.Data))
	switch pdu.Header.Cmd {
	case drdynvcCmdCapability:
		if pdu.Version < drdynvcCapsVersion1 {
			return fmt.Errorf("unsupported drdynvc capability version %d", pdu.Version)
		}
		firstCaps := !m.capsReceived
		m.capsReceived = true
		m.negotiatedCapsVersion = drdynvcCapsVersion1
		tracef("drdynvc_caps", "version=%d negotiated=%d first=%t", pdu.Version, m.negotiatedCapsVersion, firstCaps)
		if firstCaps {
			if err := m.writeStaticPayload(conn, buildDRDYNVCCapsPDU(m.negotiatedCapsVersion)); err != nil {
				return err
			}
		}
		return m.openRDPGFXChannel(conn)
	case drdynvcCmdCreate:
		if err := m.requireCaps(pdu.Header.Cmd); err != nil {
			return err
		}
		if m.pendingRDPGFXChannel && pdu.ChannelID == m.rdpgfxChannelID && pdu.Name == "" {
			if pdu.CreationCode != drdynvcCreateOK {
				m.pendingRDPGFXChannel = false
				m.hasRDPGFXChannel = false
				tracef("drdynvc_create_response", "channel=%d name=%q accepted=false code=0x%08x", pdu.ChannelID, rdpgfxDynamicChannelName, pdu.CreationCode)
				return nil
			}
			m.pendingRDPGFXChannel = false
			m.hasRDPGFXChannel = true
			m.channels[pdu.ChannelID] = rdpgfxDynamicChannelName
			tracef("drdynvc_create_response", "channel=%d name=%q accepted=true", pdu.ChannelID, rdpgfxDynamicChannelName)
			return nil
		}
		code := drdynvcCreateNoListener
		if existing := m.channels[pdu.ChannelID]; existing != "" {
			code = drdynvcCreateAlreadyExists
			tracef("drdynvc_create", "channel=%d name=%q accepted=false duplicate=%q", pdu.ChannelID, pdu.Name, existing)
		} else if pdu.Name == rdpeiDynamicChannelName {
			if m.hasRDPEIChannel {
				code = drdynvcCreateAlreadyExists
				tracef("drdynvc_create", "channel=%d name=%q accepted=false active_rdpei=%d", pdu.ChannelID, pdu.Name, m.rdpeiChannelID)
			} else {
				m.channels[pdu.ChannelID] = pdu.Name
				m.rdpeiChannelID = pdu.ChannelID
				m.hasRDPEIChannel = true
				m.touchLifecycle.Reset()
				code = drdynvcCreateOK
				tracef("drdynvc_create", "channel=%d name=%q accepted=true", pdu.ChannelID, pdu.Name)
			}
		} else if pdu.Name == rdpgfxDynamicChannelName {
			if m.hasRDPGFXChannel {
				code = drdynvcCreateAlreadyExists
				tracef("drdynvc_create", "channel=%d name=%q accepted=false active_rdpgfx=%d", pdu.ChannelID, pdu.Name, m.rdpgfxChannelID)
			} else {
				m.channels[pdu.ChannelID] = pdu.Name
				m.rdpgfxChannelID = pdu.ChannelID
				m.hasRDPGFXChannel = true
				m.rdpgfxCapsConfirmed = false
				m.rdpgfxCapability = rdpgfxCapabilitySet{}
				code = drdynvcCreateOK
				tracef("drdynvc_create", "channel=%d name=%q accepted=true", pdu.ChannelID, pdu.Name)
			}
		} else {
			tracef("drdynvc_create", "channel=%d name=%q accepted=false", pdu.ChannelID, pdu.Name)
		}
		if err := m.writeStaticPayload(conn, buildDRDYNVCCreateResponsePDU(pdu.ChannelID, code)); err != nil {
			return err
		}
		if code == drdynvcCreateOK && pdu.Name == rdpeiDynamicChannelName {
			return m.writeStaticPayload(conn, buildDRDYNVCDataPDU(pdu.ChannelID, buildRDPEISCReadyPDU(rdpeiProtocolV300, nil)))
		}
	case drdynvcCmdData:
		if err := m.requireOpenChannel(pdu.Header.Cmd, pdu.ChannelID); err != nil {
			return err
		}
		return m.handleDynamicData(conn, pdu.ChannelID, pdu.Data)
	case drdynvcCmdDataFirst:
		if err := m.requireOpenChannel(pdu.Header.Cmd, pdu.ChannelID); err != nil {
			return err
		}
		tracef("drdynvc_data_first", "channel=%d expected=%d fragment_len=%d", pdu.ChannelID, pdu.Length, len(pdu.Data))
		return m.handleDynamicDataFirst(conn, pdu.ChannelID, pdu.Length, pdu.Data)
	case drdynvcCmdClose:
		if err := m.requireCaps(pdu.Header.Cmd); err != nil {
			return err
		}
		tracef("drdynvc_close", "channel=%d rdpei=%t rdpgfx=%t", pdu.ChannelID, pdu.ChannelID == m.rdpeiChannelID, pdu.ChannelID == m.rdpgfxChannelID)
		delete(m.channels, pdu.ChannelID)
		delete(m.fragments, pdu.ChannelID)
		if pdu.ChannelID == m.rdpeiChannelID {
			m.hasRDPEIChannel = false
			m.rdpeiChannelID = 0
			m.touchLifecycle.Reset()
		}
		if pdu.ChannelID == m.rdpgfxChannelID {
			m.hasRDPGFXChannel = false
			m.pendingRDPGFXChannel = false
			m.rdpgfxChannelID = 0
			m.rdpgfxCapsConfirmed = false
			m.rdpgfxCapability = rdpgfxCapabilitySet{}
		}
	}
	return nil
}

func (m *drdynvcManager) requireCaps(cmd uint8) error {
	if !m.capsReceived {
		return fmt.Errorf("drdynvc command 0x%x before capability negotiation", cmd)
	}
	return nil
}

func (m *drdynvcManager) requireOpenChannel(cmd uint8, channelID uint32) error {
	if err := m.requireCaps(cmd); err != nil {
		return err
	}
	if _, ok := m.channels[channelID]; !ok {
		return fmt.Errorf("drdynvc command 0x%x for unopened channel %d", cmd, channelID)
	}
	return nil
}

func (m *drdynvcManager) handleDynamicDataFirst(conn net.Conn, channelID, expected uint32, data []byte) error {
	m.cleanupFragments(time.Now())
	if expected > drdynvcMaxFragmentSize {
		return fmt.Errorf("drdynvc data-first length %d exceeds maximum %d", expected, drdynvcMaxFragmentSize)
	}
	if len(data) > drdynvcMaxFragmentSize {
		return fmt.Errorf("drdynvc data-first fragment length %d exceeds maximum %d", len(data), drdynvcMaxFragmentSize)
	}
	if expected < uint32(len(data)) {
		return fmt.Errorf("drdynvc data-first length %d smaller than fragment %d", expected, len(data))
	}
	if expected == uint32(len(data)) {
		return m.handleDynamicData(conn, channelID, data)
	}
	if _, exists := m.fragments[channelID]; !exists && len(m.fragments) >= drdynvcMaxFragments {
		return fmt.Errorf("drdynvc pending fragments %d exceeds maximum %d", len(m.fragments)+1, drdynvcMaxFragments)
	}
	m.metrics.recordDVCFragment()
	m.fragments[channelID] = &drdynvcFragment{expected: expected, data: append([]byte(nil), data...), updatedAt: time.Now()}
	return nil
}

func (m *drdynvcManager) handleDynamicData(conn net.Conn, channelID uint32, data []byte) error {
	m.cleanupFragments(time.Now())
	if len(data) > drdynvcMaxFragmentSize {
		return fmt.Errorf("drdynvc data length %d exceeds maximum %d", len(data), drdynvcMaxFragmentSize)
	}
	if frag := m.fragments[channelID]; frag != nil {
		m.metrics.recordDVCFragment()
		if len(frag.data)+len(data) > drdynvcMaxFragmentSize {
			delete(m.fragments, channelID)
			return fmt.Errorf("drdynvc fragment length %d exceeds maximum %d", len(frag.data)+len(data), drdynvcMaxFragmentSize)
		}
		frag.data = append(frag.data, data...)
		frag.updatedAt = time.Now()
		if uint32(len(frag.data)) < frag.expected {
			return nil
		}
		if uint32(len(frag.data)) > frag.expected {
			delete(m.fragments, channelID)
			return fmt.Errorf("drdynvc fragment length %d exceeds expected %d", len(frag.data), frag.expected)
		}
		data = append([]byte(nil), frag.data...)
		delete(m.fragments, channelID)
	}
	switch {
	case m.hasRDPEIChannel && channelID == m.rdpeiChannelID:
		if len(data) > rdpeiMaxPDUSize {
			return fmt.Errorf("RDPEI dynamic data length %d exceeds maximum %d", len(data), rdpeiMaxPDUSize)
		}
		pdu, err := parseRDPEIPDU(data)
		if err != nil {
			return fmt.Errorf("parse RDPEI dynamic data: %w", err)
		}
		traceRDPEIPDU(pdu)
		return dispatchRDPEITouchEvent(pdu, m.sink, m.touchLifecycle)
	case m.hasRDPGFXChannel && channelID == m.rdpgfxChannelID:
		return m.handleRDPGFXData(conn, data)
	default:
		return nil
	}
}

func (m *drdynvcManager) handleRDPGFXData(conn net.Conn, data []byte) error {
	if len(data) > rdpgfxMaxPDUSize {
		return fmt.Errorf("RDPGFX dynamic data length %d exceeds maximum %d", len(data), rdpgfxMaxPDUSize)
	}
	pdu, err := parseRDPGFXPDU(data)
	if err != nil {
		return fmt.Errorf("parse RDPGFX dynamic data: %w", err)
	}
	traceRDPGFXPDU(pdu)
	if pdu.CmdID != rdpgfxCmdCapsAdvertise {
		return nil
	}
	capability, ok := negotiateRDPGFXCapability(pdu.Caps)
	if !ok {
		return fmt.Errorf("RDPGFX client advertised no supported capabilities")
	}
	m.rdpgfxCapsConfirmed = true
	m.rdpgfxCapability = capability
	tracef("rdpgfx_caps_confirm", "channel=%d version=0x%08x flags=0x%08x", m.rdpgfxChannelID, capability.Version, capability.Flags)
	return m.writeRDPGFXPayload(conn, buildRDPGFXCapsConfirmPDU(capability))
}

func (m *drdynvcManager) cleanupFragments(now time.Time) {
	if m == nil || len(m.fragments) == 0 {
		return
	}
	for channelID, frag := range m.fragments {
		if now.Sub(frag.updatedAt) > drdynvcFragmentTTL {
			delete(m.fragments, channelID)
			tracef("drdynvc_fragment_expired", "channel=%d expected=%d buffered=%d", channelID, frag.expected, len(frag.data))
		}
	}
}

func (m *drdynvcManager) startServerInitiatedChannels(conn net.Conn) error {
	if !m.enabled() {
		return nil
	}
	if !m.capsReceived {
		m.capsReceived = true
		m.negotiatedCapsVersion = drdynvcCapsVersion1
		tracef("drdynvc_caps", "version=%d negotiated=%d source=server", m.negotiatedCapsVersion, m.negotiatedCapsVersion)
		if err := m.writeStaticPayload(conn, buildDRDYNVCCapsPDU(m.negotiatedCapsVersion)); err != nil {
			return err
		}
	}
	return m.openRDPGFXChannel(conn)
}

func (m *drdynvcManager) openRDPGFXChannel(conn net.Conn) error {
	if m.hasRDPGFXChannel || m.pendingRDPGFXChannel || !m.enabled() {
		return nil
	}
	channelID := m.nextServerDVCChannelID
	m.nextServerDVCChannelID++
	m.rdpgfxChannelID = channelID
	m.pendingRDPGFXChannel = true
	m.rdpgfxCapsConfirmed = false
	m.rdpgfxCapability = rdpgfxCapabilitySet{}
	tracef("drdynvc_create_request", "channel=%d name=%q", channelID, rdpgfxDynamicChannelName)
	return m.writeStaticPayload(conn, buildDRDYNVCCreateRequestPDU(channelID, rdpgfxDynamicChannelName))
}

func (m *drdynvcManager) rdpgfxReady() bool {
	return m != nil && m.hasRDPGFXChannel && m.rdpgfxCapsConfirmed && m.rdpgfxChannelID != 0
}

func (m *drdynvcManager) rdpgfxH264Ready() bool {
	return m.rdpgfxReady() && rdpgfxCapabilitySupportsH264(m.rdpgfxCapability)
}

func (m *drdynvcManager) rdpgfxH264Status() (ready bool, version uint32, flags uint32, reason string) {
	if !m.rdpgfxReady() {
		return false, 0, 0, "rdpgfx-not-ready"
	}
	version = m.rdpgfxCapability.Version
	flags = m.rdpgfxCapability.Flags
	if !h264EnabledFromEnv() {
		return false, version, flags, "disabled-by-env"
	}
	if h264ForcedFromEnv() && version >= rdpgfxCapsVersion81 {
		return true, version, flags, "forced-by-env"
	}
	if rdpgfxCapabilitySupportsH264(m.rdpgfxCapability) {
		return true, version, flags, "ready"
	}
	return false, version, flags, "client-avc420-not-advertised"
}

func (m *drdynvcManager) writeRDPGFXPayload(conn net.Conn, payload []byte) error {
	if !m.rdpgfxReady() {
		return fmt.Errorf("RDPGFX channel is not ready")
	}
	wrapped := append([]byte{0xe0, 0x00}, payload...)
	if len(wrapped) <= drdynvcWriteChunkSize {
		return m.writeStaticPayload(conn, buildDRDYNVCDataPDU(m.rdpgfxChannelID, wrapped))
	}
	first := minInt(len(wrapped), drdynvcWriteChunkSize)
	if err := m.writeStaticPayload(conn, buildDRDYNVCDataFirstPDU(m.rdpgfxChannelID, uint32(len(wrapped)), wrapped[:first])); err != nil { // #nosec G115 -- wrapped length is bounded by allocation.
		return err
	}
	for offset := first; offset < len(wrapped); offset += drdynvcWriteChunkSize {
		end := minInt(len(wrapped), offset+drdynvcWriteChunkSize)
		if err := m.writeStaticPayload(conn, buildDRDYNVCDataPDU(m.rdpgfxChannelID, wrapped[offset:end])); err != nil {
			return err
		}
	}
	return nil
}

func (m *drdynvcManager) writeStaticPayload(conn net.Conn, payload []byte) error {
	if !m.enabled() {
		return nil
	}
	static := buildStaticVirtualChannelPDU(payload)
	body := buildMCSSendDataIndication(serverChannelID, m.staticChannelID, static)
	return writeMCSDomainPDU(conn, mcsSendDataIndicationApp, body)
}

func dispatchRDPEITouchEvent(pdu *rdpeiPDU, sink input.Sink, lifecycle *input.TouchLifecycleCoalescer) error {
	if pdu == nil || pdu.TouchEvent == nil || sink == nil {
		return nil
	}
	touchSink, ok := sink.(input.TouchSink)
	if !ok {
		return nil
	}
	for _, frame := range pdu.TouchEvent.Frames {
		contacts := make([]input.TouchContact, 0, len(frame.Contacts))
		for _, contact := range frame.Contacts {
			contacts = append(contacts, rdpeiContactToInput(contact))
		}
		if lifecycle != nil {
			contacts = lifecycle.ApplyFrame(contacts)
		}
		if len(contacts) == 0 {
			continue
		}
		if err := touchSink.TouchFrame(contacts); err != nil {
			return err
		}
	}
	return nil
}

func rdpeiContactToInput(contact rdpeiTouchContact) input.TouchContact {
	out := input.TouchContact{ID: contact.ContactID, X: int(contact.X), Y: int(contact.Y), Flags: rdpeiTouchFlagsToInput(contact.Flags)}
	if contact.ContactRect != nil {
		out.Rect = &input.TouchRect{
			Left:   int(contact.ContactRect.Left),
			Top:    int(contact.ContactRect.Top),
			Right:  int(contact.ContactRect.Right),
			Bottom: int(contact.ContactRect.Bottom),
		}
	}
	if contact.Orientation != nil {
		orientation := *contact.Orientation
		out.Orientation = &orientation
	}
	if contact.Pressure != nil {
		pressure := *contact.Pressure
		out.Pressure = &pressure
	}
	return out
}

func rdpeiTouchFlagsToInput(flags uint32) input.TouchFlags {
	var out input.TouchFlags
	if flags&rdpeiContactFlagDown != 0 {
		out |= input.TouchDown
	}
	if flags&rdpeiContactFlagUpdate != 0 {
		out |= input.TouchUpdate
	}
	if flags&rdpeiContactFlagUp != 0 {
		out |= input.TouchUp
	}
	if flags&rdpeiContactFlagInRange != 0 {
		out |= input.TouchInRange
	}
	if flags&rdpeiContactFlagInContact != 0 {
		out |= input.TouchInContact
	}
	if flags&rdpeiContactFlagCanceled != 0 {
		out |= input.TouchCanceled
	}
	return out
}

func traceRDPEIPDU(pdu *rdpeiPDU) {
	switch {
	case pdu == nil:
	case pdu.CSReady != nil:
		tracef("rdpei_cs_ready", "flags=0x%08x version=0x%08x max_touch_contacts=%d", pdu.CSReady.Flags, pdu.CSReady.ProtocolVersion, pdu.CSReady.MaxTouchContacts)
	case pdu.TouchEvent != nil:
		contacts := 0
		for _, frame := range pdu.TouchEvent.Frames {
			contacts += len(frame.Contacts)
		}
		tracef("rdpei_touch", "frames=%d contacts=%d encode_time=%d", len(pdu.TouchEvent.Frames), contacts, pdu.TouchEvent.EncodeTime)
	case pdu.DismissTouch != nil:
		tracef("rdpei_dismiss_hovering", "contact_id=%d", pdu.DismissTouch.ContactID)
	}
}

func parseStaticVirtualChannelPDU(data []byte) (*staticVirtualChannelPDU, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("short static virtual channel PDU")
	}
	length := binary.LittleEndian.Uint32(data[0:4])
	if length > drdynvcMaxStaticPayload {
		return nil, fmt.Errorf("static virtual channel length %d exceeds maximum %d", length, drdynvcMaxStaticPayload)
	}
	flags := binary.LittleEndian.Uint32(data[4:8])
	payload := data[8:]
	if len(payload) > drdynvcMaxStaticPayload {
		return nil, fmt.Errorf("static virtual channel payload length %d exceeds maximum %d", len(payload), drdynvcMaxStaticPayload)
	}
	if flags&channelFlagLast != 0 && length != uint32(len(payload)) {
		return nil, fmt.Errorf("static virtual channel length mismatch: header=%d payload=%d", length, len(payload))
	}
	return &staticVirtualChannelPDU{Length: length, Flags: flags, Data: payload}, nil
}

func buildStaticVirtualChannelPDU(payload []byte) []byte {
	out := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint32(out[0:4], uint32(len(payload))) // #nosec G115 -- payload length is bounded by allocation.
	binary.LittleEndian.PutUint32(out[4:8], channelFlagFirst|channelFlagLast)
	copy(out[8:], payload)
	return out
}

func parseDRDYNVCPDU(data []byte) (*drdynvcPDU, error) {
	if len(data) < 1 {
		return nil, fmt.Errorf("short drdynvc PDU")
	}
	if len(data) > drdynvcMaxPDUSize {
		return nil, fmt.Errorf("drdynvc PDU length %d exceeds maximum %d", len(data), drdynvcMaxPDUSize)
	}
	header := drdynvcHeader{CbChID: data[0] & 0x03, Sp: (data[0] >> 2) & 0x03, Cmd: (data[0] >> 4) & 0x0f}
	cur := &bytesCursor{data: data[1:]}
	pdu := &drdynvcPDU{Header: header}
	var err error
	switch header.Cmd {
	case drdynvcCmdCapability:
		if cur.remaining() < 3 {
			return nil, fmt.Errorf("short drdynvc capability PDU")
		}
		_, _ = cur.readByte() // pad
		pdu.Version, err = cur.readUint16LE()
	case drdynvcCmdCreate:
		pdu.ChannelID, err = cur.readDVCChannelID(header.CbChID)
		if err != nil {
			return nil, err
		}
		pdu.Data = cur.rest()
		if len(pdu.Data) == 4 {
			pdu.CreationCode = binary.LittleEndian.Uint32(pdu.Data[0:4])
		} else {
			nameCur := &bytesCursor{data: pdu.Data}
			pdu.Name, err = nameCur.readNullTerminatedString()
		}
	case drdynvcCmdData:
		pdu.ChannelID, err = cur.readDVCChannelID(header.CbChID)
		if err == nil {
			pdu.Data = cur.rest()
		}
	case drdynvcCmdDataFirst:
		pdu.ChannelID, err = cur.readDVCChannelID(header.CbChID)
		if err != nil {
			return nil, err
		}
		pdu.Length, err = cur.readDVCLength(header.Sp)
		if err == nil {
			pdu.Data = cur.rest()
		}
	case drdynvcCmdClose:
		pdu.ChannelID, err = cur.readDVCChannelID(header.CbChID)
	default:
		return nil, fmt.Errorf("unsupported drdynvc command 0x%x", header.Cmd)
	}
	if err != nil {
		return nil, err
	}
	return pdu, nil
}

var (
	drdynvcCapsVersion1PDU = [...]byte{(drdynvcHeader{Cmd: drdynvcCmdCapability}).serialize(), 0, byte(drdynvcCapsVersion1), byte(drdynvcCapsVersion1 >> 8)}

	drdynvcCreateOKResponseSmall            = buildDRDYNVCCreateResponseSmallTable(drdynvcCreateOK)
	drdynvcCreateAlreadyExistsResponseSmall = buildDRDYNVCCreateResponseSmallTable(drdynvcCreateAlreadyExists)
	drdynvcCreateNoListenerResponseSmall    = buildDRDYNVCCreateResponseSmallTable(drdynvcCreateNoListener)
	rdpgfxCreateRequestSmall                = buildDRDYNVCCreateRequestSmallTable(rdpgfxDynamicChannelName)
)

func buildDRDYNVCCreateResponseSmallTable(creationCode uint32) [256][6]byte {
	var table [256][6]byte
	for channelID := range table {
		out := table[channelID][:]
		out[0] = (drdynvcHeader{CbChID: 0, Cmd: drdynvcCmdCreate}).serialize()
		out[1] = byte(channelID)
		binary.LittleEndian.PutUint32(out[2:6], creationCode)
	}
	return table
}

func buildDRDYNVCCreateRequestSmallTable(name string) [256][1 + 1 + len(rdpgfxDynamicChannelName) + 1]byte {
	var table [256][1 + 1 + len(rdpgfxDynamicChannelName) + 1]byte
	for channelID := range table {
		out := table[channelID][:]
		out[0] = (drdynvcHeader{CbChID: 0, Cmd: drdynvcCmdCreate}).serialize()
		out[1] = byte(channelID)
		copy(out[2:], name)
	}
	return table
}

func buildDRDYNVCCapsPDU(version uint16) []byte {
	if version == drdynvcCapsVersion1 {
		return drdynvcCapsVersion1PDU[:]
	}
	out := make([]byte, 4)
	out[0] = drdynvcHeader{Cmd: drdynvcCmdCapability}.serialize()
	out[1] = 0
	binary.LittleEndian.PutUint16(out[2:4], version)
	return out
}

func buildDRDYNVCCreateResponsePDU(channelID uint32, creationCode uint32) []byte {
	if channelID <= 0xff {
		smallID := byte(channelID)
		switch creationCode {
		case drdynvcCreateOK:
			return drdynvcCreateOKResponseSmall[smallID][:]
		case drdynvcCreateAlreadyExists:
			return drdynvcCreateAlreadyExistsResponseSmall[smallID][:]
		case drdynvcCreateNoListener:
			return drdynvcCreateNoListenerResponseSmall[smallID][:]
		}
	}
	cb := drdynvcCbChID(channelID)
	out := make([]byte, 1+dvcChannelIDLen(cb)+4)
	out[0] = (drdynvcHeader{CbChID: cb, Cmd: drdynvcCmdCreate}).serialize()
	off := writeDVCChannelID(out[1:], cb, channelID) + 1
	binary.LittleEndian.PutUint32(out[off:off+4], creationCode)
	return out
}

func buildDRDYNVCCreateRequestPDU(channelID uint32, name string) []byte {
	if channelID <= 0xff && name == rdpgfxDynamicChannelName {
		return rdpgfxCreateRequestSmall[byte(channelID)][:]
	}
	cb := drdynvcCbChID(channelID)
	out := make([]byte, 1+dvcChannelIDLen(cb)+len(name)+1)
	out[0] = (drdynvcHeader{CbChID: cb, Cmd: drdynvcCmdCreate}).serialize()
	off := writeDVCChannelID(out[1:], cb, channelID) + 1
	copy(out[off:], name)
	return out
}

func buildDRDYNVCDataPDU(channelID uint32, data []byte) []byte {
	cb := drdynvcCbChID(channelID)
	out := make([]byte, 1+dvcChannelIDLen(cb)+len(data))
	out[0] = (drdynvcHeader{CbChID: cb, Cmd: drdynvcCmdData}).serialize()
	off := writeDVCChannelID(out[1:], cb, channelID) + 1
	copy(out[off:], data)
	return out
}

func buildDRDYNVCDataFirstPDU(channelID uint32, length uint32, data []byte) []byte {
	cb := drdynvcCbChID(channelID)
	sp := drdynvcSpLength(length)
	out := make([]byte, 1+dvcChannelIDLen(cb)+dvcLengthLen(sp)+len(data))
	out[0] = (drdynvcHeader{CbChID: cb, Sp: sp, Cmd: drdynvcCmdDataFirst}).serialize()
	off := writeDVCChannelID(out[1:], cb, channelID) + 1
	off += writeDVCLength(out[off:], sp, length)
	copy(out[off:], data)
	return out
}

func drdynvcCbChID(channelID uint32) uint8 {
	switch {
	case channelID <= 0xff:
		return 0
	case channelID <= 0xffff:
		return 1
	default:
		return 2
	}
}

func (h drdynvcHeader) serialize() byte {
	return (h.CbChID & 0x03) | ((h.Sp & 0x03) << 2) | ((h.Cmd & 0x0f) << 4)
}

func appendDVCChannelID(out []byte, cb uint8, channelID uint32) []byte {
	start := len(out)
	out = append(out, make([]byte, dvcChannelIDLen(cb))...)
	writeDVCChannelID(out[start:], cb, channelID)
	return out
}

func dvcChannelIDLen(cb uint8) int {
	if cb == 0 {
		return 1
	}
	if cb == 1 {
		return 2
	}
	return 4
}

func writeDVCChannelID(out []byte, cb uint8, channelID uint32) int {
	switch cb {
	case 0:
		out[0] = byte(channelID)
		return 1
	case 1:
		binary.LittleEndian.PutUint16(out[0:2], uint16(channelID)) // #nosec G115 -- cb selected from channelID range.
		return 2
	default:
		binary.LittleEndian.PutUint32(out[0:4], channelID)
		return 4
	}
}

func drdynvcSpLength(length uint32) uint8 {
	switch {
	case length <= 0xff:
		return 0
	case length <= 0xffff:
		return 1
	default:
		return 2
	}
}

func appendDVCLength(out []byte, sp uint8, length uint32) []byte {
	start := len(out)
	out = append(out, make([]byte, dvcLengthLen(sp))...)
	writeDVCLength(out[start:], sp, length)
	return out
}

func dvcLengthLen(sp uint8) int {
	if sp == 0 {
		return 1
	}
	if sp == 1 {
		return 2
	}
	return 4
}

func writeDVCLength(out []byte, sp uint8, length uint32) int {
	switch sp {
	case 0:
		out[0] = byte(length)
		return 1
	case 1:
		binary.LittleEndian.PutUint16(out[0:2], uint16(length)) // #nosec G115 -- sp selected from length range.
		return 2
	default:
		binary.LittleEndian.PutUint32(out[0:4], length)
		return 4
	}
}

type bytesCursor struct {
	data []byte
	off  int
}

func (c *bytesCursor) remaining() int { return len(c.data) - c.off }
func (c *bytesCursor) rest() []byte {
	out := append([]byte(nil), c.data[c.off:]...)
	c.off = len(c.data)
	return out
}
func (c *bytesCursor) readByte() (byte, error) {
	if c.remaining() < 1 {
		return 0, fmt.Errorf("buffer exhausted")
	}
	b := c.data[c.off]
	c.off++
	return b, nil
}
func (c *bytesCursor) readUint16LE() (uint16, error) {
	if c.remaining() < 2 {
		return 0, fmt.Errorf("buffer exhausted")
	}
	v := binary.LittleEndian.Uint16(c.data[c.off : c.off+2])
	c.off += 2
	return v, nil
}
func (c *bytesCursor) readUint32LE() (uint32, error) {
	if c.remaining() < 4 {
		return 0, fmt.Errorf("buffer exhausted")
	}
	v := binary.LittleEndian.Uint32(c.data[c.off : c.off+4])
	c.off += 4
	return v, nil
}
func (c *bytesCursor) readDVCChannelID(cb uint8) (uint32, error) {
	switch cb {
	case 0:
		b, err := c.readByte()
		return uint32(b), err
	case 1:
		v, err := c.readUint16LE()
		return uint32(v), err
	case 2:
		return c.readUint32LE()
	default:
		return 0, fmt.Errorf("invalid drdynvc channel ID size %d", cb)
	}
}
func (c *bytesCursor) readDVCLength(sp uint8) (uint32, error) {
	switch sp {
	case 0:
		b, err := c.readByte()
		return uint32(b), err
	case 1:
		v, err := c.readUint16LE()
		return uint32(v), err
	case 2:
		return c.readUint32LE()
	default:
		return 0, fmt.Errorf("invalid drdynvc length size %d", sp)
	}
}
func (c *bytesCursor) readNullTerminatedString() (string, error) {
	idx := bytes.IndexByte(c.data[c.off:], 0)
	if idx < 0 {
		return "", fmt.Errorf("missing null terminator")
	}
	name := string(c.data[c.off : c.off+idx])
	c.off += idx + 1
	if c.remaining() != 0 {
		return "", fmt.Errorf("trailing bytes after null-terminated string")
	}
	return name, nil
}
