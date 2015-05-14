package engine

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/nicholaskh/golib/server"
	log "github.com/nicholaskh/log4go"
	"github.com/ugorji/go/codec"
)

type Client struct {
	*server.Client
	dec   *codec.Decoder
	codec *codec.MsgpackHandle
}

func newClient(cli *server.Client) *Client {
	this := new(Client)
	this.Client = cli
	this.codec = &codec.MsgpackHandle{}
	this.codec.MapType = reflect.TypeOf(map[string]interface{}(nil))
	this.codec.SliceType = reflect.TypeOf([]interface{}(nil))
	this.codec.RawToString = false
	this.dec = codec.NewDecoder(cli.Conn, this.codec)

	return this
}

func (c *Client) decodeEntries() ([]FluentRecordSet, error) {
	v := []interface{}{nil, nil, nil}
	err := c.dec.Decode(&v)

	if err != nil {
		return nil, err
	}
	tag, ok := v[0].([]byte)
	if !ok {
		return nil, errors.New("Failed to decode tag field")
	}

	var retval []FluentRecordSet
	switch timestamp_or_entries := v[1].(type) {
	case uint64:
		timestamp := timestamp_or_entries
		data, ok := v[2].(map[string]interface{})
		if !ok {
			return nil, errors.New("Failed to decode data field")
		}
		coerceInPlace(data)
		retval = []FluentRecordSet{
			{
				Tag: string(tag), // XXX: byte => rune
				Records: []TinyFluentRecord{
					{
						Timestamp: timestamp,
						Data:      data,
					},
				},
			},
		}
	case float64:
		timestamp := uint64(timestamp_or_entries)
		data, ok := v[2].(map[string]interface{})
		if !ok {
			return nil, errors.New("Failed to decode data field")
		}
		retval = []FluentRecordSet{
			{
				Tag: string(tag), // XXX: byte => rune
				Records: []TinyFluentRecord{
					{
						Timestamp: timestamp,
						Data:      data,
					},
				},
			},
		}
	case []interface{}:
		if !ok {
			return nil, errors.New("Unexpected payload format")
		}
		recordSet, err := c.decodeRecordSet(tag, timestamp_or_entries)
		if err != nil {
			return nil, err
		}
		retval = []FluentRecordSet{recordSet}
	case []byte:
		entries := make([]interface{}, 10)
		err := codec.NewDecoderBytes(timestamp_or_entries, c.codec).Decode(&entries)
		log.Info(timestamp_or_entries)
		log.Info("aaa", entries)
		if err != nil {
			return nil, err
		}
		recordSet, err := c.decodeRecordSet(tag, entries)
		if err != nil {
			return nil, err
		}
		retval = []FluentRecordSet{recordSet}
	default:
		return nil, errors.New(fmt.Sprintf("Unknown type: %t", timestamp_or_entries))
	}
	//	atomic.AddInt64(&c.input.entries, int64(len(retval)))
	return retval, nil
}

func (c *Client) decodeRecordSet(tag []byte, entries []interface{}) (FluentRecordSet, error) {
	records := make([]TinyFluentRecord, len(entries))
	for i, _entry := range entries {
		entry, ok := _entry.([]interface{})
		if !ok {
			return FluentRecordSet{}, errors.New("Failed to decode recordSet")
		}
		timestamp, ok := entry[0].(uint64)
		if !ok {
			return FluentRecordSet{}, errors.New("Failed to decode timestamp field")
		}
		data, ok := entry[1].(map[string]interface{})
		if !ok {
			return FluentRecordSet{}, errors.New("Failed to decode data field")
		}
		coerceInPlace(data)
		records[i] = TinyFluentRecord{
			Timestamp: timestamp,
			Data:      data,
		}
	}
	return FluentRecordSet{
		Tag:     string(tag), // XXX: byte => rune
		Records: records,
	}, nil
}

func coerceInPlace(data map[string]interface{}) {
	for k, v := range data {
		switch v_ := v.(type) {
		case []byte:
			data[k] = string(v_) // XXX: byte => rune
		case map[string]interface{}:
			coerceInPlace(v_)
		}
	}
}
