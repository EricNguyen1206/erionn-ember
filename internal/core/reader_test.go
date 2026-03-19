package core

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestRESPReader_ReadCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *MemKVCmd
		wantErr bool
	}{
		{
			name:  "Single RESP Array Command",
			input: "*2\r\n$3\r\nGET\r\n$4\r\nNAME\r\n",
			want: &MemKVCmd{
				Cmd:  "GET",
				Args: []string{"NAME"},
			},
			wantErr: false,
		},
		{
			name:  "Single RESP Array Command with Integer",
			input: "*3\r\n$3\r\nSET\r\n$4\r\nNAME\r\n$4\r\nERIC\r\n",
			want: &MemKVCmd{
				Cmd:  "SET",
				Args: []string{"NAME", "ERIC"},
			},
			wantErr: false,
		},
		{
			name:  "Inline PING Command",
			input: "PING\r\n",
			want: &MemKVCmd{
				Cmd:  "PING",
				Args: []string{},
			},
			wantErr: false,
		},
		{
			name:  "Inline SET Command",
			input: "SET key value\r\n",
			want: &MemKVCmd{
				Cmd:  "SET",
				Args: []string{"key", "value"},
			},
			wantErr: false,
		},
		{
			name:    "Malformed Array Length",
			input:   "*abc\r\n",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Short Bulk String",
			input:   "*2\r\n$3\r\nGET\r\n$4\r\nKEY\r\n", // KEY is 3 chars, but header says 4
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Empty Input",
			input:   "",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRESPReader(bytes.NewReader([]byte(tt.input)))
			got, err := r.ReadCommand()
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadCommand() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRESPReader_Pipelining(t *testing.T) {
	input := "*1\r\n$4\r\nPING\r\n*2\r\n$3\r\nGET\r\n$1\r\nA\r\n"
	r := NewRESPReader(bytes.NewReader([]byte(input)))

	// First command
	cmd1, err := r.ReadCommand()
	if err != nil {
		t.Fatalf("Failed to read first command: %v", err)
	}
	want1 := &MemKVCmd{Cmd: "PING", Args: []string{}}
	if !reflect.DeepEqual(cmd1, want1) {
		t.Errorf("First command got = %v, want %v", cmd1, want1)
	}

	// Second command
	cmd2, err := r.ReadCommand()
	if err != nil {
		t.Fatalf("Failed to read second command: %v", err)
	}
	want2 := &MemKVCmd{Cmd: "GET", Args: []string{"A"}}
	if !reflect.DeepEqual(cmd2, want2) {
		t.Errorf("Second command got = %v, want %v", cmd2, want2)
	}

	// EOF
	_, err = r.ReadCommand()
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
}

func TestRESPReader_EOF_MidCommand(t *testing.T) {
	input := "*2\r\n$3\r\nGET\r\n$4\r\n" // Missing bulk string value
	r := NewRESPReader(bytes.NewReader([]byte(input)))

	_, err := r.ReadCommand()
	if err == nil {
		t.Errorf("Expected error for partial command, got nil")
	}
}
