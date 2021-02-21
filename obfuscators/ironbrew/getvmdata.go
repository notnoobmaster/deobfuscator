package ironbrew

import (
	// assigned to _ because go:embed doesn't work without requiring embed
	_ "embed"
	"errors"
	"strconv"
	"strings"

	"github.com/notnoobmaster/beautifier"
	"github.com/notnoobmaster/deobfuscator/obfuscators/ironbrew/opcodemap"
	"github.com/yuin/gopher-lua/ast"
	"github.com/yuin/gopher-lua/parse"
)

// 1: Arg 2: Protos 3: Instructions
const (
	parameters byte = iota
	constants
	instructions
	prototypes
	lineinfo
)

type settings struct {
	BytecodeCompress bool
	PreserveLineInfo bool
}

type vmdata struct {
	Loop        *ast.IfStmt
	Settings    settings
	Deserialize *ast.FunctionExpr
	Opcodemap	map[int]*opcodemap.Instruction
	Order       []byte
	Pos         int
	Key         byte
	Bool        int
	Float       int
	String      int
	Env         string
	Upvalues    string
	Stack		string
	Inst 		string
	InstPtr		string
	Bytecode    []byte
}

//go:embed "patterns/constants.lua"
var strConstants string
var astConstants []ast.Stmt

//go:embed "patterns/instructions.lua"
var strInstructions string
var astInstructions []ast.Stmt

var strParameters = "Chunk[3] = gBits8();"
var astParameters []ast.Stmt

var strPrototypes = "for Idx=1,gBits32() do Functions[Idx-1]=Deserialize();end;"
var astPrototypes []ast.Stmt

var strLineinfo = "for Idx=1,gBits32() do Lines[Idx]=gBits32();end;"
var astLineinfo []ast.Stmt

func (data *vmdata) order(chunk []ast.Stmt) bool {
	for _, stmt := range chunk {
		switch stmt.(type) {
		case *ast.NumberForStmt:
			if success, exprs, _ := beautifier.Match([]ast.Stmt{stmt}, astConstants); success {
				data.Bool, _ = strconv.Atoi(exprs[0].(*ast.NumberExpr).Value)
				data.Float, _ = strconv.Atoi(exprs[1].(*ast.NumberExpr).Value)
				data.String, _ = strconv.Atoi(exprs[2].(*ast.NumberExpr).Value)
				data.Order = append(data.Order, constants)
				break
			}
			if success, _, _ := beautifier.Match([]ast.Stmt{stmt}, astInstructions); success {
				data.Order = append(data.Order, instructions)
				break
			}
			if success, _, _ := beautifier.Match([]ast.Stmt{stmt}, astPrototypes); success {
				data.Order = append(data.Order, prototypes)
				break
			}
			if success, _, _ := beautifier.Match([]ast.Stmt{stmt}, astLineinfo); success {
				data.Order = append(data.Order, lineinfo)
				break
			}
		case *ast.AssignStmt:
			if success, _, _ := beautifier.Match([]ast.Stmt{stmt}, astParameters); success {
				data.Order = append(data.Order, parameters)
			}
		}
	}

	return true
}

//go:embed patterns/compressed.lua
var strCompressed string
var astCompressed []ast.Stmt

func (data *vmdata) compressed(chunk []ast.Stmt) bool {
	success, exprs, _ := beautifier.Match(chunk, astCompressed)
	if !success {
		return success
	}
	byteString := exprs[0].(*ast.StringExpr).Value
	if bytecode, err := decompress(byteString); err == nil {
		data.Settings.BytecodeCompress = true
		data.Bytecode = bytecode
		return success
	}
	return false
}

//go:embed patterns/uncompressed.lua
var strUncompressed string
var astUncompressed []ast.Stmt

func (data *vmdata) uncompressed(chunk []ast.Stmt) bool {
	success, exprs, _ := beautifier.Match(chunk, astUncompressed)
	if !success {
		return success
	}
	data.Bytecode = []byte(exprs[0].(*ast.StringExpr).Value)
	return success
}

//go:embed patterns/normal.lua
var strNormal string
var astNormal []ast.Stmt

func (data *vmdata) normal(chunk []ast.Stmt) bool {
	success, exprs, stmts := beautifier.Match(chunk, astNormal)
	if !success {
		return success
	}
	key, _ := strconv.Atoi(exprs[0].(*ast.NumberExpr).Value)
	data.Key = byte(key)
	data.Deserialize = exprs[1].(*ast.FunctionExpr)
	
	data.InstPtr = exprs[2].(*ast.IdentExpr).Value
	data.Stack = exprs[3].(*ast.IdentExpr).Value
	data.Inst = exprs[4].(*ast.IdentExpr).Value

	data.Upvalues = exprs[5].(*ast.IdentExpr).Value
	data.Env = exprs[6].(*ast.IdentExpr).Value

	data.Loop = stmts[0].(*ast.IfStmt)
	return success
}

//go:embed patterns/lineinfo.lua
var strWithlineinfo string
var astWithlineinfo []ast.Stmt

func (data *vmdata) withlineinfo(chunk []ast.Stmt) bool {

	return true
}

func (data *vmdata) getVmdata(chunk []ast.Stmt) (err error) {
	if !(data.compressed(chunk) || data.uncompressed(chunk)) {
		return errors.New("Couldn't get VM bytecode")
	}

	if !(data.normal(chunk) || data.withlineinfo(chunk)) {
		return errors.New("Couldn't get VM data")
	}

	if !data.order(data.Deserialize.Stmts) {
		return errors.New("Couldn't get order")
	}

	return nil
}

func compile(str string) ([]ast.Stmt, error) {
	chunk, err := parse.Parse(strings.NewReader(str), "")
	if err != nil {
		return nil, err
	}
	return chunk, nil
}

func initVmdata() error {
	toCompile := map[string]*[]ast.Stmt{
		strConstants:    &astConstants,
		strInstructions: &astInstructions,
		strPrototypes:   &astPrototypes,
		strLineinfo:     &astLineinfo,
		strParameters:   &astParameters,

		strCompressed:   &astCompressed,
		strUncompressed: &astUncompressed,

		strNormal:       &astNormal,
		strWithlineinfo: &astWithlineinfo,
	}
	for str, a := range toCompile {
		chunk, err := parse.Parse(strings.NewReader(str), "")
		if err != nil {
			return err
		}
		*a = chunk
	}
	return nil
}
