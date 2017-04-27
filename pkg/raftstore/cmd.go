// Copyright 2016 DeepFabric, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package raftstore

import (
	"errors"

	"github.com/deepfabric/elasticell/pkg/pb/errorpb"
	"github.com/deepfabric/elasticell/pkg/pb/metapb"
	"github.com/deepfabric/elasticell/pkg/pb/raftcmdpb"
	"golang.org/x/net/context"
)

var (
	errStaleCMD           = errors.New("stale command")
	errStaleEpoch         = errors.New("stale epoch")
	errNotLeader          = errors.New("NotLeader")
	errCellNotFound       = errors.New("cell not found")
	errMissingUUIDCMD     = errors.New("missing request uuid")
	errLargeRaftEntrySize = errors.New("entry is too large")
	errKeyNotInCell       = errors.New("key not in cell")

	infoStaleCMD = new(errorpb.StaleCommand)
)

type splitCheckResult struct {
	cellID   uint64
	epoch    metapb.CellEpoch
	splitKey []byte
}

func buildTerm(term uint64, resp *raftcmdpb.RaftCMDResponse) {
	resp.Header.CurrentTerm = term
}

func buildUUID(uuid []byte, resp *raftcmdpb.RaftCMDResponse) {
	if resp.Header.UUID != nil {
		resp.Header.UUID = uuid
	}
}

func errorOtherCMDResp(err error) *raftcmdpb.RaftCMDResponse {
	resp := errorBaseResp(nil, 0)
	resp.Header.Error.Message = err.Error()
	return resp
}

func errorPbResp(err *errorpb.Error, uuid []byte, currentTerm uint64) *raftcmdpb.RaftCMDResponse {
	resp := errorBaseResp(uuid, currentTerm)
	resp.Header.Error = *err

	return resp
}

func errorStaleCMDResp(uuid []byte, currentTerm uint64) *raftcmdpb.RaftCMDResponse {
	resp := errorBaseResp(uuid, currentTerm)

	resp.Header.Error.Message = errStaleCMD.Error()
	resp.Header.Error.StaleCommand = infoStaleCMD

	return resp
}

func errorStaleEpochResp(uuid []byte, currentTerm uint64, newCells ...metapb.Cell) *raftcmdpb.RaftCMDResponse {
	resp := errorBaseResp(uuid, currentTerm)

	resp.Header.Error.Message = errStaleCMD.Error()
	resp.Header.Error.StaleEpoch = &errorpb.StaleEpoch{
		NewCells: newCells,
	}

	return resp
}

func errorBaseResp(uuid []byte, currentTerm uint64) *raftcmdpb.RaftCMDResponse {
	resp := new(raftcmdpb.RaftCMDResponse)
	resp.Header = new(raftcmdpb.RaftResponseHeader)
	buildTerm(currentTerm, resp)
	buildUUID(uuid, resp)

	return resp
}

type cmd struct {
	req *raftcmdpb.RaftCMDRequest
	cb  func(*raftcmdpb.RaftCMDResponse)
}

func newCMD(req *raftcmdpb.RaftCMDRequest, cb func(*raftcmdpb.RaftCMDResponse)) *cmd {
	return &cmd{
		req: req,
		cb:  cb,
	}
}

func (c *cmd) resp(resp *raftcmdpb.RaftCMDResponse) {
	if c.cb != nil {
		c.cb(resp)
	}
}

func (c *cmd) respLargeRaftEntrySize(cellID uint64, size uint64, term uint64) {
	err := &errorpb.RaftEntryTooLarge{
		CellID:    cellID,
		EntrySize: size,
	}

	rsp := errorPbResp(&errorpb.Error{
		Message:           errLargeRaftEntrySize.Error(),
		RaftEntryTooLarge: err,
	}, c.getUUID(), term)

	c.resp(rsp)
}

func (c *cmd) respOtherError(err error) {
	rsp := errorOtherCMDResp(err)
	c.resp(rsp)
}

func (c *cmd) respNotLeader(cellID uint64, term uint64, leader *metapb.Peer) {
	err := &errorpb.NotLeader{
		CellID: cellID,
	}

	if leader != nil {
		err.Leader = *leader
	}

	rsp := errorPbResp(&errorpb.Error{
		Message:   errNotLeader.Error(),
		NotLeader: err,
	}, c.getUUID(), term)

	c.resp(rsp)
}

func (c *cmd) getUUID() []byte {
	return c.req.Header.UUID
}

func (pr *PeerReplicate) execReadLocal(cmd *cmd) {
	pr.doExecReadCmd(cmd)
}

func (pr *PeerReplicate) execReadIndex(cmd *cmd) {
	pr.rn.ReadIndex(context.TODO(), cmd.getUUID())
	pr.pendingReads.push(cmd)
}
