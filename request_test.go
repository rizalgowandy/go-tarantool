package tarantool_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tarantool/go-iproto"
	. "github.com/tarantool/go-tarantool/v2"
	"github.com/vmihailenco/msgpack/v5"
)

const invalidSpaceMsg = "invalid space"
const invalidIndexMsg = "invalid index"

const invalidSpace uint32 = 2
const invalidIndex uint32 = 2
const validSpace uint32 = 1    // Any valid value != default.
const validIndex uint32 = 3    // Any valid value != default.
const validExpr = "any string" // We don't check the value here.
const validKey = "foo"         // Any string.
const defaultSpace uint32 = 0  // And valid too.
const defaultIndex uint32 = 0  // And valid too.

const defaultIsolationLevel = DefaultIsolationLevel
const defaultTimeout = 0

const validTimeout = 500 * time.Millisecond

var validStmt *Prepared = &Prepared{StatementID: 1, Conn: &Connection{}}

var validProtocolInfo ProtocolInfo = ProtocolInfo{
	Version:  ProtocolVersion(3),
	Features: []iproto.Feature{iproto.IPROTO_FEATURE_STREAMS},
}

type ValidSchemeResolver struct {
	nameUseSupported   bool
	spaceResolverCalls int
	indexResolverCalls int
}

func (r *ValidSchemeResolver) ResolveSpace(s interface{}) (uint32, error) {
	r.spaceResolverCalls++

	var spaceNo uint32
	if no, ok := s.(uint32); ok {
		spaceNo = no
	} else {
		spaceNo = defaultSpace
	}
	if spaceNo == invalidSpace {
		return 0, errors.New(invalidSpaceMsg)
	}
	return spaceNo, nil
}

func (r *ValidSchemeResolver) ResolveIndex(i interface{}, spaceNo uint32) (uint32, error) {
	r.indexResolverCalls++

	var indexNo uint32
	if no, ok := i.(uint32); ok {
		indexNo = no
	} else {
		indexNo = defaultIndex
	}
	if indexNo == invalidIndex {
		return 0, errors.New(invalidIndexMsg)
	}
	return indexNo, nil
}

func (r *ValidSchemeResolver) NamesUseSupported() bool {
	return r.nameUseSupported
}

var resolver ValidSchemeResolver

func assertBodyCall(t testing.TB, requests []Request, errorMsg string) {
	t.Helper()

	const errBegin = "An unexpected Request.Body() "
	for _, req := range requests {
		var reqBuf bytes.Buffer
		enc := msgpack.NewEncoder(&reqBuf)

		err := req.Body(&resolver, enc)
		if err != nil && errorMsg != "" && err.Error() != errorMsg {
			t.Errorf(errBegin+"error %q expected %q", err.Error(), errorMsg)
		}
		if err != nil && errorMsg == "" {
			t.Errorf(errBegin+"error %q", err.Error())
		}
		if err == nil && errorMsg != "" {
			t.Errorf(errBegin+"result, expected error %q", errorMsg)
		}
	}
}

func assertBodyEqual(t testing.TB, reference []byte, req Request) {
	t.Helper()

	var reqBuf bytes.Buffer
	reqEnc := msgpack.NewEncoder(&reqBuf)

	err := req.Body(&resolver, reqEnc)
	if err != nil {
		t.Fatalf("An unexpected Response.Body() error: %q", err.Error())
	}

	reqBody := reqBuf.Bytes()
	if !bytes.Equal(reqBody, reference) {
		t.Errorf("Encoded request %v != reference %v", reqBody, reference)
	}
}

func getTestOps() *Operations {
	operations := NewOperations().
		Add(1, 2).
		Subtract(3, 4).
		BitwiseAnd(5, 6).
		BitwiseOr(7, 8).
		BitwiseXor(9, 1).
		BitwiseXor(9, 1). // The duplication is for test purposes.
		Splice(2, 3, 1, "!!").
		Insert(4, 5).
		Delete(6, 7).
		Assign(8, 9)
	return operations
}

func TestRequestsValidSpaceAndIndex(t *testing.T) {
	requests := []Request{
		NewSelectRequest(validSpace),
		NewSelectRequest(validSpace).Index(validIndex),
		NewUpdateRequest(validSpace),
		NewUpdateRequest(validSpace).Index(validIndex),
		NewUpsertRequest(validSpace),
		NewInsertRequest(validSpace),
		NewReplaceRequest(validSpace),
		NewDeleteRequest(validSpace),
		NewDeleteRequest(validSpace).Index(validIndex),
	}

	assertBodyCall(t, requests, "")
}

func TestRequestsInvalidSpace(t *testing.T) {
	requests := []Request{
		NewSelectRequest(invalidSpace).Index(validIndex),
		NewSelectRequest(invalidSpace),
		NewUpdateRequest(invalidSpace).Index(validIndex),
		NewUpdateRequest(invalidSpace),
		NewUpsertRequest(invalidSpace),
		NewInsertRequest(invalidSpace),
		NewReplaceRequest(invalidSpace),
		NewDeleteRequest(invalidSpace).Index(validIndex),
		NewDeleteRequest(invalidSpace),
	}

	assertBodyCall(t, requests, invalidSpaceMsg)
}

func TestRequestsInvalidIndex(t *testing.T) {
	requests := []Request{
		NewSelectRequest(validSpace).Index(invalidIndex),
		NewUpdateRequest(validSpace).Index(invalidIndex),
		NewDeleteRequest(validSpace).Index(invalidIndex),
	}

	assertBodyCall(t, requests, invalidIndexMsg)
}

func TestRequestsTypes(t *testing.T) {
	tests := []struct {
		req   Request
		rtype iproto.Type
	}{
		{req: NewSelectRequest(validSpace), rtype: iproto.IPROTO_SELECT},
		{req: NewUpdateRequest(validSpace), rtype: iproto.IPROTO_UPDATE},
		{req: NewUpsertRequest(validSpace), rtype: iproto.IPROTO_UPSERT},
		{req: NewInsertRequest(validSpace), rtype: iproto.IPROTO_INSERT},
		{req: NewReplaceRequest(validSpace), rtype: iproto.IPROTO_REPLACE},
		{req: NewDeleteRequest(validSpace), rtype: iproto.IPROTO_DELETE},
		{req: NewCallRequest(validExpr), rtype: iproto.IPROTO_CALL},
		{req: NewCall16Request(validExpr), rtype: iproto.IPROTO_CALL_16},
		{req: NewCall17Request(validExpr), rtype: iproto.IPROTO_CALL},
		{req: NewEvalRequest(validExpr), rtype: iproto.IPROTO_EVAL},
		{req: NewExecuteRequest(validExpr), rtype: iproto.IPROTO_EXECUTE},
		{req: NewPingRequest(), rtype: iproto.IPROTO_PING},
		{req: NewPrepareRequest(validExpr), rtype: iproto.IPROTO_PREPARE},
		{req: NewUnprepareRequest(validStmt), rtype: iproto.IPROTO_PREPARE},
		{req: NewExecutePreparedRequest(validStmt), rtype: iproto.IPROTO_EXECUTE},
		{req: NewBeginRequest(), rtype: iproto.IPROTO_BEGIN},
		{req: NewCommitRequest(), rtype: iproto.IPROTO_COMMIT},
		{req: NewRollbackRequest(), rtype: iproto.IPROTO_ROLLBACK},
		{req: NewIdRequest(validProtocolInfo), rtype: iproto.IPROTO_ID},
		{req: NewBroadcastRequest(validKey), rtype: iproto.IPROTO_CALL},
		{req: NewWatchOnceRequest(validKey), rtype: iproto.IPROTO_WATCH_ONCE},
	}

	for _, test := range tests {
		if rtype := test.req.Type(); rtype != test.rtype {
			t.Errorf("An invalid request type 0x%x, expected 0x%x",
				rtype, test.rtype)
		}
	}
}

func TestRequestsAsync(t *testing.T) {
	tests := []struct {
		req   Request
		async bool
	}{
		{req: NewSelectRequest(validSpace), async: false},
		{req: NewUpdateRequest(validSpace), async: false},
		{req: NewUpsertRequest(validSpace), async: false},
		{req: NewInsertRequest(validSpace), async: false},
		{req: NewReplaceRequest(validSpace), async: false},
		{req: NewDeleteRequest(validSpace), async: false},
		{req: NewCallRequest(validExpr), async: false},
		{req: NewCall16Request(validExpr), async: false},
		{req: NewCall17Request(validExpr), async: false},
		{req: NewEvalRequest(validExpr), async: false},
		{req: NewExecuteRequest(validExpr), async: false},
		{req: NewPingRequest(), async: false},
		{req: NewPrepareRequest(validExpr), async: false},
		{req: NewUnprepareRequest(validStmt), async: false},
		{req: NewExecutePreparedRequest(validStmt), async: false},
		{req: NewBeginRequest(), async: false},
		{req: NewCommitRequest(), async: false},
		{req: NewRollbackRequest(), async: false},
		{req: NewIdRequest(validProtocolInfo), async: false},
		{req: NewBroadcastRequest(validKey), async: false},
		{req: NewWatchOnceRequest(validKey), async: false},
	}

	for _, test := range tests {
		if async := test.req.Async(); async != test.async {
			t.Errorf("An invalid async %t, expected %t", async, test.async)
		}
	}
}

func TestRequestsCtx_default(t *testing.T) {
	tests := []struct {
		req      Request
		expected context.Context
	}{
		{req: NewSelectRequest(validSpace), expected: nil},
		{req: NewUpdateRequest(validSpace), expected: nil},
		{req: NewUpsertRequest(validSpace), expected: nil},
		{req: NewInsertRequest(validSpace), expected: nil},
		{req: NewReplaceRequest(validSpace), expected: nil},
		{req: NewDeleteRequest(validSpace), expected: nil},
		{req: NewCallRequest(validExpr), expected: nil},
		{req: NewCall16Request(validExpr), expected: nil},
		{req: NewCall17Request(validExpr), expected: nil},
		{req: NewEvalRequest(validExpr), expected: nil},
		{req: NewExecuteRequest(validExpr), expected: nil},
		{req: NewPingRequest(), expected: nil},
		{req: NewPrepareRequest(validExpr), expected: nil},
		{req: NewUnprepareRequest(validStmt), expected: nil},
		{req: NewExecutePreparedRequest(validStmt), expected: nil},
		{req: NewBeginRequest(), expected: nil},
		{req: NewCommitRequest(), expected: nil},
		{req: NewRollbackRequest(), expected: nil},
		{req: NewIdRequest(validProtocolInfo), expected: nil},
		{req: NewBroadcastRequest(validKey), expected: nil},
		{req: NewWatchOnceRequest(validKey), expected: nil},
	}

	for _, test := range tests {
		if ctx := test.req.Ctx(); ctx != test.expected {
			t.Errorf("An invalid ctx %t, expected %t", ctx, test.expected)
		}
	}
}

func TestRequestsCtx_setter(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		req      Request
		expected context.Context
	}{
		{req: NewSelectRequest(validSpace).Context(ctx), expected: ctx},
		{req: NewUpdateRequest(validSpace).Context(ctx), expected: ctx},
		{req: NewUpsertRequest(validSpace).Context(ctx), expected: ctx},
		{req: NewInsertRequest(validSpace).Context(ctx), expected: ctx},
		{req: NewReplaceRequest(validSpace).Context(ctx), expected: ctx},
		{req: NewDeleteRequest(validSpace).Context(ctx), expected: ctx},
		{req: NewCallRequest(validExpr).Context(ctx), expected: ctx},
		{req: NewCall16Request(validExpr).Context(ctx), expected: ctx},
		{req: NewCall17Request(validExpr).Context(ctx), expected: ctx},
		{req: NewEvalRequest(validExpr).Context(ctx), expected: ctx},
		{req: NewExecuteRequest(validExpr).Context(ctx), expected: ctx},
		{req: NewPingRequest().Context(ctx), expected: ctx},
		{req: NewPrepareRequest(validExpr).Context(ctx), expected: ctx},
		{req: NewUnprepareRequest(validStmt).Context(ctx), expected: ctx},
		{req: NewExecutePreparedRequest(validStmt).Context(ctx), expected: ctx},
		{req: NewBeginRequest().Context(ctx), expected: ctx},
		{req: NewCommitRequest().Context(ctx), expected: ctx},
		{req: NewRollbackRequest().Context(ctx), expected: ctx},
		{req: NewIdRequest(validProtocolInfo).Context(ctx), expected: ctx},
		{req: NewBroadcastRequest(validKey).Context(ctx), expected: ctx},
		{req: NewWatchOnceRequest(validKey).Context(ctx), expected: ctx},
	}

	for _, test := range tests {
		if ctx := test.req.Ctx(); ctx != test.expected {
			t.Errorf("An invalid ctx %t, expected %t", ctx, test.expected)
		}
	}
}

func TestPingRequestDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplPingBody(refEnc)
	if err != nil {
		t.Fatalf("An unexpected RefImplPingBody() error: %q", err.Error())
	}

	req := NewPingRequest()
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestSelectRequestDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplSelectBody(refEnc, &resolver, validSpace, defaultIndex, 0, 0xFFFFFFFF,
		IterAll, []interface{}{}, nil, false)
	if err != nil {
		t.Fatalf("An unexpected RefImplSelectBody() error %q", err.Error())
	}

	req := NewSelectRequest(validSpace)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestSelectRequestSpaceByName(t *testing.T) {
	var refBuf bytes.Buffer

	resolver.nameUseSupported = true

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplSelectBody(refEnc, &resolver, "valid", defaultIndex, 0, 0xFFFFFFFF,
		IterAll, []interface{}{}, nil, false)
	if err != nil {
		t.Fatalf("An unexpected RefImplSelectBody() error %q", err.Error())
	}

	req := NewSelectRequest("valid")
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestSelectRequestIndexByName(t *testing.T) {
	var refBuf bytes.Buffer

	resolver.nameUseSupported = true

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplSelectBody(refEnc, &resolver, defaultSpace, "valid", 0, 0xFFFFFFFF,
		IterAll, []interface{}{}, nil, false)
	if err != nil {
		t.Fatalf("An unexpected RefImplSelectBody() error %q", err.Error())
	}

	req := NewSelectRequest(defaultSpace)
	req.Index("valid")
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestSelectRequestDefaultIteratorEqIfKey(t *testing.T) {
	var refBuf bytes.Buffer
	key := []interface{}{uint(18)}

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplSelectBody(refEnc, &resolver, validSpace, defaultIndex, 0, 0xFFFFFFFF,
		IterEq, key, nil, false)
	if err != nil {
		t.Fatalf("An unexpected RefImplSelectBody() error %q", err.Error())
	}

	req := NewSelectRequest(validSpace).
		Key(key)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestSelectRequestIteratorNotChangedIfKey(t *testing.T) {
	var refBuf bytes.Buffer
	key := []interface{}{uint(678)}
	const iter = IterGe

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplSelectBody(refEnc, &resolver, validSpace, defaultIndex, 0, 0xFFFFFFFF,
		iter, key, nil, false)
	if err != nil {
		t.Fatalf("An unexpected RefImplSelectBody() error %q", err.Error())
	}

	req := NewSelectRequest(validSpace).
		Iterator(iter).
		Key(key)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestSelectRequestSetters(t *testing.T) {
	const offset = 4
	const limit = 5
	const iter = IterLt
	key := []interface{}{uint(36)}
	afterBytes := []byte{0x1, 0x2, 0x3}
	afterKey := []interface{}{uint(13)}
	var refBufAfterBytes, refBufAfterKey bytes.Buffer

	refEncAfterBytes := msgpack.NewEncoder(&refBufAfterBytes)
	err := RefImplSelectBody(refEncAfterBytes, &resolver, validSpace, validIndex, offset,
		limit, iter, key, afterBytes, true)
	if err != nil {
		t.Fatalf("An unexpected RefImplSelectBody() error %s", err)
	}

	refEncAfterKey := msgpack.NewEncoder(&refBufAfterKey)
	err = RefImplSelectBody(refEncAfterKey, &resolver, validSpace, validIndex, offset,
		limit, iter, key, afterKey, true)
	if err != nil {
		t.Fatalf("An unexpected RefImplSelectBody() error %s", err)
	}

	reqAfterBytes := NewSelectRequest(validSpace).
		Index(validIndex).
		Offset(offset).
		Limit(limit).
		Iterator(iter).
		Key(key).
		After(afterBytes).
		FetchPos(true)
	reqAfterKey := NewSelectRequest(validSpace).
		Index(validIndex).
		Offset(offset).
		Limit(limit).
		Iterator(iter).
		Key(key).
		After(afterKey).
		FetchPos(true)

	assertBodyEqual(t, refBufAfterBytes.Bytes(), reqAfterBytes)
	assertBodyEqual(t, refBufAfterKey.Bytes(), reqAfterKey)
}

func TestInsertRequestDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplInsertBody(refEnc, &resolver, validSpace, []interface{}{})
	if err != nil {
		t.Fatalf("An unexpected RefImplInsertBody() error: %q", err.Error())
	}

	req := NewInsertRequest(validSpace)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestInsertRequestSpaceByName(t *testing.T) {
	var refBuf bytes.Buffer

	resolver.nameUseSupported = true

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplInsertBody(refEnc, &resolver, "valid", []interface{}{})
	if err != nil {
		t.Fatalf("An unexpected RefImplInsertBody() error: %q", err.Error())
	}

	req := NewInsertRequest("valid")
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestInsertRequestSetters(t *testing.T) {
	tuple := []interface{}{uint(24)}
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplInsertBody(refEnc, &resolver, validSpace, tuple)
	if err != nil {
		t.Fatalf("An unexpected RefImplInsertBody() error: %q", err.Error())
	}

	req := NewInsertRequest(validSpace).
		Tuple(tuple)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestReplaceRequestDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplReplaceBody(refEnc, &resolver, validSpace, []interface{}{})
	if err != nil {
		t.Fatalf("An unexpected RefImplReplaceBody() error: %q", err.Error())
	}

	req := NewReplaceRequest(validSpace)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestReplaceRequestSpaceByName(t *testing.T) {
	var refBuf bytes.Buffer

	resolver.nameUseSupported = true

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplReplaceBody(refEnc, &resolver, "valid", []interface{}{})
	if err != nil {
		t.Fatalf("An unexpected RefImplReplaceBody() error: %q", err.Error())
	}

	req := NewReplaceRequest("valid")
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestReplaceRequestSetters(t *testing.T) {
	tuple := []interface{}{uint(99)}
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplReplaceBody(refEnc, &resolver, validSpace, tuple)
	if err != nil {
		t.Fatalf("An unexpected RefImplReplaceBody() error: %q", err.Error())
	}

	req := NewReplaceRequest(validSpace).
		Tuple(tuple)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestDeleteRequestDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplDeleteBody(refEnc, &resolver, validSpace, defaultIndex, []interface{}{})
	if err != nil {
		t.Fatalf("An unexpected RefImplDeleteBody() error: %q", err.Error())
	}

	req := NewDeleteRequest(validSpace)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestDeleteRequestSpaceByName(t *testing.T) {
	var refBuf bytes.Buffer

	resolver.nameUseSupported = true

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplDeleteBody(refEnc, &resolver, "valid", defaultIndex, []interface{}{})
	if err != nil {
		t.Fatalf("An unexpected RefImplDeleteBody() error: %q", err.Error())
	}

	req := NewDeleteRequest("valid")
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestDeleteRequestIndexByName(t *testing.T) {
	var refBuf bytes.Buffer

	resolver.nameUseSupported = true

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplDeleteBody(refEnc, &resolver, defaultSpace, "valid", []interface{}{})
	if err != nil {
		t.Fatalf("An unexpected RefImplDeleteBody() error: %q", err.Error())
	}

	req := NewDeleteRequest(defaultSpace)
	req.Index("valid")
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestDeleteRequestSetters(t *testing.T) {
	key := []interface{}{uint(923)}
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplDeleteBody(refEnc, &resolver, validSpace, validIndex, key)
	if err != nil {
		t.Fatalf("An unexpected RefImplDeleteBody() error: %q", err.Error())
	}

	req := NewDeleteRequest(validSpace).
		Index(validIndex).
		Key(key)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestUpdateRequestDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplUpdateBody(refEnc, &resolver, validSpace, defaultIndex,
		[]interface{}{}, nil)
	if err != nil {
		t.Fatalf("An unexpected RefImplUpdateBody() error: %q", err.Error())
	}

	req := NewUpdateRequest(validSpace)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestUpdateRequestSpaceByName(t *testing.T) {
	var refBuf bytes.Buffer

	resolver.nameUseSupported = true

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplUpdateBody(refEnc, &resolver, "valid", defaultIndex,
		[]interface{}{}, nil)
	if err != nil {
		t.Fatalf("An unexpected RefImplUpdateBody() error: %q", err.Error())
	}

	req := NewUpdateRequest("valid")
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestUpdateRequestIndexByName(t *testing.T) {
	var refBuf bytes.Buffer

	resolver.nameUseSupported = true

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplUpdateBody(refEnc, &resolver, defaultSpace, "valid",
		[]interface{}{}, nil)
	if err != nil {
		t.Fatalf("An unexpected RefImplUpdateBody() error: %q", err.Error())
	}

	req := NewUpdateRequest(defaultSpace)
	req.Index("valid")
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestUpdateRequestSetters(t *testing.T) {
	key := []interface{}{uint(44)}
	reqOps := getTestOps()
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplUpdateBody(refEnc, &resolver, validSpace, validIndex, key, reqOps)
	if err != nil {
		t.Fatalf("An unexpected RefImplUpdateBody() error: %q", err.Error())
	}

	req := NewUpdateRequest(validSpace).
		Index(validIndex).
		Key(key).
		Operations(reqOps)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestUpsertRequestDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplUpsertBody(refEnc, &resolver, validSpace, []interface{}{}, nil)
	if err != nil {
		t.Fatalf("An unexpected RefImplUpsertBody() error: %q", err.Error())
	}

	req := NewUpsertRequest(validSpace)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestUpsertRequestSpaceByName(t *testing.T) {
	var refBuf bytes.Buffer

	resolver.nameUseSupported = true

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplUpsertBody(refEnc, &resolver, "valid", []interface{}{}, nil)
	if err != nil {
		t.Fatalf("An unexpected RefImplUpsertBody() error: %q", err.Error())
	}

	req := NewUpsertRequest("valid")
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestUpsertRequestSetters(t *testing.T) {
	tuple := []interface{}{uint(64)}
	reqOps := getTestOps()
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplUpsertBody(refEnc, &resolver, validSpace, tuple, reqOps)
	if err != nil {
		t.Fatalf("An unexpected RefImplUpsertBody() error: %q", err.Error())
	}

	req := NewUpsertRequest(validSpace).
		Tuple(tuple).
		Operations(reqOps)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestCallRequestsDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplCallBody(refEnc, validExpr, []interface{}{})
	if err != nil {
		t.Fatalf("An unexpected RefImplCallBody() error: %q", err.Error())
	}

	req := NewCallRequest(validExpr)
	req16 := NewCall16Request(validExpr)
	req17 := NewCall17Request(validExpr)
	assertBodyEqual(t, refBuf.Bytes(), req)
	assertBodyEqual(t, refBuf.Bytes(), req16)
	assertBodyEqual(t, refBuf.Bytes(), req17)
}

func TestCallRequestsSetters(t *testing.T) {
	args := []interface{}{uint(34)}
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplCallBody(refEnc, validExpr, args)
	if err != nil {
		t.Fatalf("An unexpected RefImplCallBody() error: %q", err.Error())
	}

	req := NewCallRequest(validExpr).
		Args(args)
	req16 := NewCall16Request(validExpr).
		Args(args)
	req17 := NewCall17Request(validExpr).
		Args(args)
	assertBodyEqual(t, refBuf.Bytes(), req)
	assertBodyEqual(t, refBuf.Bytes(), req16)
	assertBodyEqual(t, refBuf.Bytes(), req17)
}

func TestEvalRequestDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplEvalBody(refEnc, validExpr, []interface{}{})
	if err != nil {
		t.Fatalf("An unexpected RefImplEvalBody() error: %q", err.Error())
	}

	req := NewEvalRequest(validExpr)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestEvalRequestSetters(t *testing.T) {
	args := []interface{}{uint(34), int(12)}
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplEvalBody(refEnc, validExpr, args)
	if err != nil {
		t.Fatalf("An unexpected RefImplEvalBody() error: %q", err.Error())
	}

	req := NewEvalRequest(validExpr).
		Args(args)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestExecuteRequestDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplExecuteBody(refEnc, validExpr, []interface{}{})
	if err != nil {
		t.Fatalf("An unexpected RefImplExecuteBody() error: %q", err.Error())
	}

	req := NewExecuteRequest(validExpr)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestExecuteRequestSetters(t *testing.T) {
	args := []interface{}{uint(11)}
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplExecuteBody(refEnc, validExpr, args)
	if err != nil {
		t.Fatalf("An unexpected RefImplExecuteBody() error: %q", err.Error())
	}

	req := NewExecuteRequest(validExpr).
		Args(args)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestPrepareRequestDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplPrepareBody(refEnc, validExpr)
	if err != nil {
		t.Fatalf("An unexpected RefImplPrepareBody() error: %q", err.Error())
	}

	req := NewPrepareRequest(validExpr)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestUnprepareRequestDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplUnprepareBody(refEnc, *validStmt)
	if err != nil {
		t.Fatalf("An unexpected RefImplUnprepareBody() error: %q", err.Error())
	}

	req := NewUnprepareRequest(validStmt)
	assert.Equal(t, req.Conn(), validStmt.Conn)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestExecutePreparedRequestSetters(t *testing.T) {
	args := []interface{}{uint(11)}
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplExecutePreparedBody(refEnc, *validStmt, args)
	if err != nil {
		t.Fatalf("An unexpected RefImplExecutePreparedBody() error: %q", err.Error())
	}

	req := NewExecutePreparedRequest(validStmt).
		Args(args)
	assert.Equal(t, req.Conn(), validStmt.Conn)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestExecutePreparedRequestDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplExecutePreparedBody(refEnc, *validStmt, []interface{}{})
	if err != nil {
		t.Fatalf("An unexpected RefImplExecutePreparedBody() error: %q", err.Error())
	}

	req := NewExecutePreparedRequest(validStmt)
	assert.Equal(t, req.Conn(), validStmt.Conn)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestBeginRequestDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplBeginBody(refEnc, defaultIsolationLevel, defaultTimeout)
	if err != nil {
		t.Fatalf("An unexpected RefImplBeginBody() error: %q", err.Error())
	}

	req := NewBeginRequest()
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestBeginRequestSetters(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplBeginBody(refEnc, ReadConfirmedLevel, validTimeout)
	if err != nil {
		t.Fatalf("An unexpected RefImplBeginBody() error: %q", err.Error())
	}

	req := NewBeginRequest().TxnIsolation(ReadConfirmedLevel).Timeout(validTimeout)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestCommitRequestDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplCommitBody(refEnc)
	if err != nil {
		t.Fatalf("An unexpected RefImplCommitBody() error: %q", err.Error())
	}

	req := NewCommitRequest()
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestRollbackRequestDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplRollbackBody(refEnc)
	if err != nil {
		t.Fatalf("An unexpected RefImplRollbackBody() error: %q", err.Error())
	}

	req := NewRollbackRequest()
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestBroadcastRequestDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	expectedArgs := []interface{}{validKey}
	err := RefImplCallBody(refEnc, "box.broadcast", expectedArgs)
	if err != nil {
		t.Fatalf("An unexpected RefImplCallBody() error: %q", err.Error())
	}

	req := NewBroadcastRequest(validKey)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestBroadcastRequestSetters(t *testing.T) {
	value := []interface{}{uint(34), int(12)}
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	expectedArgs := []interface{}{validKey, value}
	err := RefImplCallBody(refEnc, "box.broadcast", expectedArgs)
	if err != nil {
		t.Fatalf("An unexpected RefImplCallBody() error: %q", err.Error())
	}

	req := NewBroadcastRequest(validKey).Value(value)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestWatchOnceRequestDefaultValues(t *testing.T) {
	var refBuf bytes.Buffer

	refEnc := msgpack.NewEncoder(&refBuf)
	err := RefImplWatchOnceBody(refEnc, validKey)
	if err != nil {
		t.Fatalf("An unexpected RefImplCallBody() error: %q", err.Error())
	}

	req := NewWatchOnceRequest(validKey)
	assertBodyEqual(t, refBuf.Bytes(), req)
}

func TestResponseDecode(t *testing.T) {
	header := Header{}
	data := bytes.NewBuffer([]byte{'v', '2'})
	baseExample, err := NewPingRequest().Response(header, data)
	assert.NoError(t, err)

	tests := []struct {
		req      Request
		expected Response
	}{
		{req: NewSelectRequest(validSpace), expected: &SelectResponse{}},
		{req: NewUpdateRequest(validSpace), expected: baseExample},
		{req: NewUpsertRequest(validSpace), expected: baseExample},
		{req: NewInsertRequest(validSpace), expected: baseExample},
		{req: NewReplaceRequest(validSpace), expected: baseExample},
		{req: NewDeleteRequest(validSpace), expected: baseExample},
		{req: NewCallRequest(validExpr), expected: baseExample},
		{req: NewCall16Request(validExpr), expected: baseExample},
		{req: NewCall17Request(validExpr), expected: baseExample},
		{req: NewEvalRequest(validExpr), expected: baseExample},
		{req: NewExecuteRequest(validExpr), expected: &ExecuteResponse{}},
		{req: NewPingRequest(), expected: baseExample},
		{req: NewPrepareRequest(validExpr), expected: &PrepareResponse{}},
		{req: NewUnprepareRequest(validStmt), expected: baseExample},
		{req: NewExecutePreparedRequest(validStmt), expected: &ExecuteResponse{}},
		{req: NewBeginRequest(), expected: baseExample},
		{req: NewCommitRequest(), expected: baseExample},
		{req: NewRollbackRequest(), expected: baseExample},
		{req: NewIdRequest(validProtocolInfo), expected: baseExample},
		{req: NewBroadcastRequest(validKey), expected: baseExample},
		{req: NewWatchOnceRequest(validKey), expected: baseExample},
	}

	for _, test := range tests {
		buf := bytes.NewBuffer([]byte{})
		enc := msgpack.NewEncoder(buf)

		enc.EncodeMapLen(1)
		enc.EncodeUint8(uint8(iproto.IPROTO_DATA))
		enc.Encode([]interface{}{'v', '2'})

		resp, err := test.req.Response(header, bytes.NewBuffer(buf.Bytes()))
		assert.NoError(t, err)
		assert.True(t, fmt.Sprintf("%T", resp) ==
			fmt.Sprintf("%T", test.expected))
		assert.Equal(t, header, resp.Header())

		decodedInterface, err := resp.Decode()
		assert.NoError(t, err)
		assert.Equal(t, []interface{}{'v', '2'}, decodedInterface)
	}
}

func TestResponseDecodeTyped(t *testing.T) {
	header := Header{}
	data := bytes.NewBuffer([]byte{'v', '2'})
	baseExample, err := NewPingRequest().Response(header, data)
	assert.NoError(t, err)

	tests := []struct {
		req      Request
		expected Response
	}{
		{req: NewSelectRequest(validSpace), expected: &SelectResponse{}},
		{req: NewUpdateRequest(validSpace), expected: baseExample},
		{req: NewUpsertRequest(validSpace), expected: baseExample},
		{req: NewInsertRequest(validSpace), expected: baseExample},
		{req: NewReplaceRequest(validSpace), expected: baseExample},
		{req: NewDeleteRequest(validSpace), expected: baseExample},
		{req: NewCallRequest(validExpr), expected: baseExample},
		{req: NewCall16Request(validExpr), expected: baseExample},
		{req: NewCall17Request(validExpr), expected: baseExample},
		{req: NewEvalRequest(validExpr), expected: baseExample},
		{req: NewExecuteRequest(validExpr), expected: &ExecuteResponse{}},
		{req: NewPingRequest(), expected: baseExample},
		{req: NewPrepareRequest(validExpr), expected: &PrepareResponse{}},
		{req: NewUnprepareRequest(validStmt), expected: baseExample},
		{req: NewExecutePreparedRequest(validStmt), expected: &ExecuteResponse{}},
		{req: NewBeginRequest(), expected: baseExample},
		{req: NewCommitRequest(), expected: baseExample},
		{req: NewRollbackRequest(), expected: baseExample},
		{req: NewIdRequest(validProtocolInfo), expected: baseExample},
		{req: NewBroadcastRequest(validKey), expected: baseExample},
		{req: NewWatchOnceRequest(validKey), expected: baseExample},
	}

	for _, test := range tests {
		buf := bytes.NewBuffer([]byte{})
		enc := msgpack.NewEncoder(buf)

		enc.EncodeMapLen(1)
		enc.EncodeUint8(uint8(iproto.IPROTO_DATA))
		enc.EncodeBytes([]byte{'v', '2'})

		resp, err := test.req.Response(header, bytes.NewBuffer(buf.Bytes()))
		assert.NoError(t, err)
		assert.True(t, fmt.Sprintf("%T", resp) ==
			fmt.Sprintf("%T", test.expected))
		assert.Equal(t, header, resp.Header())

		var decoded []byte
		err = resp.DecodeTyped(&decoded)
		assert.NoError(t, err)
		assert.Equal(t, []byte{'v', '2'}, decoded)
	}
}

type stubSchemeResolver struct {
	space interface{}
}

func (r stubSchemeResolver) ResolveSpace(s interface{}) (uint32, error) {
	if id, ok := r.space.(uint32); ok {
		return id, nil
	}
	if _, ok := r.space.(string); ok {
		return 0, nil
	}
	return 0, fmt.Errorf("stub error message: %v", r.space)
}

func (stubSchemeResolver) ResolveIndex(i interface{}, spaceNo uint32) (uint32, error) {
	return 0, nil
}

func (r stubSchemeResolver) NamesUseSupported() bool {
	_, ok := r.space.(string)
	return ok
}

func TestEncodeSpace(t *testing.T) {
	tests := []struct {
		name string
		res  stubSchemeResolver
		err  string
		out  []byte
	}{
		{
			name: "string space",
			res:  stubSchemeResolver{"test"},
			out:  []byte{0x5E, 0xA4, 0x74, 0x65, 0x73, 0x74},
		},
		{
			name: "empty string",
			res:  stubSchemeResolver{""},
			out:  []byte{0x5E, 0xA0},
		},
		{
			name: "numeric 524",
			res:  stubSchemeResolver{uint32(524)},
			out:  []byte{0x10, 0xCD, 0x02, 0x0C},
		},
		{
			name: "numeric zero",
			res:  stubSchemeResolver{uint32(0)},
			out:  []byte{0x10, 0x00},
		},
		{
			name: "numeric max value",
			res:  stubSchemeResolver{^uint32(0)},
			out:  []byte{0x10, 0xCE, 0xFF, 0xFF, 0xFF, 0xFF},
		},
		{
			name: "resolve error",
			res:  stubSchemeResolver{false},
			err:  "stub error message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			enc := msgpack.NewEncoder(&buf)

			err := EncodeSpace(tt.res, enc, tt.res.space)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tt.out, buf.Bytes())
		})
	}
}
