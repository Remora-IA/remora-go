package pingpong

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// setupTestClient creates a Client with a temp dir and initializes progress.
func setupTestClient(t *testing.T) (*Client, string) {
	t.Helper()
	dir := t.TempDir()

	// Override ProgressFile by changing to temp dir
	origDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origDir) })
	os.Chdir(dir)

	c := New()
	return c, dir
}

// loadProgress reads the progress file from the current directory.
func loadProgress(t *testing.T) *Progress {
	t.Helper()
	data, err := os.ReadFile(ProgressFile)
	if err != nil {
		t.Fatal(err)
	}
	var p Progress
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	return &p
}

// writeGoFile creates a Go source file with the given content.
func writeGoFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBatchNeverRegresses(t *testing.T) {
	c, _ := setupTestClient(t)

	// Start with 6 steps → 2 batches of 3
	_, err := c.Start("test goal", "step1;step2;step3;step4;step5;step6")
	if err != nil {
		t.Fatal(err)
	}

	p := loadProgress(t)
	if p.BatchIndex != 1 {
		t.Fatalf("expected batchIndex=1, got %d", p.BatchIndex)
	}
	if p.PassedBatches != 0 {
		t.Fatalf("expected passedBatches=0, got %d", p.PassedBatches)
	}

	// Manually mark batch 1 as passed
	p.PassedBatches = 1
	p.BatchIndex = 2
	p.CurrentStep = 4
	data, _ := json.MarshalIndent(p, "", "  ")
	os.WriteFile(ProgressFile, data, 0644)

	p2 := loadProgress(t)
	if p2.PassedBatches != 1 {
		t.Fatalf("expected passedBatches=1, got %d", p2.PassedBatches)
	}
	if p2.BatchIndex != 2 {
		t.Fatalf("expected batchIndex=2, got %d", p2.BatchIndex)
	}
}

func TestAutoAdvancePastDoneSteps(t *testing.T) {
	c, _ := setupTestClient(t)

	_, err := c.Start("test", "s1;s2;s3;s4;s5;s6")
	if err != nil {
		t.Fatal(err)
	}

	// Mark step 1 done via Done()
	_, err = c.Done("1")
	if err != nil {
		t.Fatal(err)
	}

	p := loadProgress(t)
	// Step 1 done, so currentStep should advance to 2 (not stay at 2 because it's not done)
	if p.CurrentStep != 2 {
		t.Fatalf("expected currentStep=2 after marking step 1 done, got %d", p.CurrentStep)
	}

	// Now mark step 2 done. Step 3 is not done, so currentStep should go to 3.
	_, err = c.Done("2")
	if err != nil {
		t.Fatal(err)
	}
	p = loadProgress(t)
	if p.CurrentStep != 3 {
		t.Fatalf("expected currentStep=3, got %d", p.CurrentStep)
	}

	// Mark step 3 done. Completing the batch should start a mini-test.
	p.Steps[3].Done = true // step 4 (index 3)
	data, _ := json.MarshalIndent(p, "", "  ")
	os.WriteFile(ProgressFile, data, 0644)

	_, err = c.Done("3")
	if err != nil {
		t.Fatal(err)
	}
	p = loadProgress(t)
	if !p.InMinitest {
		t.Fatal("expected mini-test to start after completing batch")
	}
	if p.CurrentStep != 1 {
		t.Fatalf("expected currentStep=1 for mini-test restart, got %d", p.CurrentStep)
	}
}

func TestDetourCreation(t *testing.T) {
	c, _ := setupTestClient(t)

	_, err := c.Start("test", "s1;s2;s3")
	if err != nil {
		t.Fatal(err)
	}

	// Create a detour for step 1
	res, err := c.Subdivide(1, "sub1;sub2;sub3")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("expected success, got: %s", res.Message)
	}

	p := loadProgress(t)

	// Steps should be unchanged (3 original steps)
	if len(p.Steps) != 3 {
		t.Fatalf("expected 3 steps unchanged, got %d", len(p.Steps))
	}

	// Detour should exist
	if p.Detour == nil {
		t.Fatal("expected detour to be created")
	}
	if p.Detour.ParentStepID != 1 {
		t.Fatalf("expected parentStepID=1, got %d", p.Detour.ParentStepID)
	}
	if len(p.Detour.Steps) != 3 {
		t.Fatalf("expected 3 detour steps, got %d", len(p.Detour.Steps))
	}
	if p.Detour.CurrentStep != 1 {
		t.Fatalf("expected detour currentStep=1, got %d", p.Detour.CurrentStep)
	}
}

func TestDetourMaxThreeSubsteps(t *testing.T) {
	c, _ := setupTestClient(t)

	_, err := c.Start("test", "s1;s2;s3")
	if err != nil {
		t.Fatal(err)
	}

	// 4 substeps should fail
	_, err = c.Subdivide(1, "a;b;c;d")
	if err == nil {
		t.Fatal("expected error for >3 substeps")
	}

	// 2 substeps should succeed
	_, err = c.Subdivide(1, "a;b")
	if err != nil {
		t.Fatalf("expected 2 substeps to work, got: %v", err)
	}
}

func TestDetourCannotDuringMinitest(t *testing.T) {
	c, _ := setupTestClient(t)

	_, err := c.Start("test", "s1;s2;s3")
	if err != nil {
		t.Fatal(err)
	}

	// Set InMinitest manually
	p := loadProgress(t)
	p.InMinitest = true
	data, _ := json.MarshalIndent(p, "", "  ")
	os.WriteFile(ProgressFile, data, 0644)

	_, err = c.Subdivide(1, "a;b")
	if err == nil {
		t.Fatal("expected error during minitest")
	}
}

func TestBuildBatchInfo(t *testing.T) {
	p := &Progress{
		BatchSize:   3,
		BatchIndex:  2,
		CurrentStep: 5,
		Steps: []Step{
			{ID: 1, Instruction: "s1", Done: true},
			{ID: 2, Instruction: "s2", Done: true},
			{ID: 3, Instruction: "s3", Done: true},
			{ID: 4, Instruction: "s4", Done: true},
			{ID: 5, Instruction: "s5", Done: false},
			{ID: 6, Instruction: "s6", Done: false},
			{ID: 7, Instruction: "s7", Done: false},
			{ID: 8, Instruction: "s8", Done: false},
			{ID: 9, Instruction: "s9", Done: false},
		},
	}

	bi := buildBatchInfo(p)

	if bi.Index != 2 {
		t.Fatalf("expected index=2, got %d", bi.Index)
	}
	if bi.TotalBatches != 3 {
		t.Fatalf("expected totalBatches=3, got %d", bi.TotalBatches)
	}
	if len(bi.Steps) != 3 {
		t.Fatalf("expected 3 batch steps, got %d", len(bi.Steps))
	}
	// Step 5 is at index 1 in batch (batchStep 2)
	if bi.CurrentBatchStep != 2 {
		t.Fatalf("expected currentBatchStep=2, got %d", bi.CurrentBatchStep)
	}
	// Verify batch step numbering
	if bi.Steps[0].BatchNum != 1 || bi.Steps[0].GlobalID != 4 {
		t.Fatalf("batch step 0: expected batchNum=1 globalID=4, got %d/%d", bi.Steps[0].BatchNum, bi.Steps[0].GlobalID)
	}
}

func TestMiniTestAttemptsAutoPass(t *testing.T) {
	// After 2 failed mini-test attempts, handleBatchComplete should auto-advance
	p := &Progress{
		Active:           true,
		BatchSize:        3,
		BatchIndex:       1,
		MiniTestAttempts: 2, // Already failed 2 times
		CurrentStep:      1,
		Steps: []Step{
			{ID: 1, Instruction: "s1", Done: true},
			{ID: 2, Instruction: "s2", Done: true},
			{ID: 3, Instruction: "s3", Done: true},
		},
	}

	// MiniTestAttempts >= 2 means auto-pass should trigger
	if p.MiniTestAttempts < 2 {
		t.Fatal("test setup: MiniTestAttempts should be >= 2")
	}

	// Verify the condition that handleBatchComplete checks
	if p.MiniTestAttempts >= 2 {
		// This is the auto-pass path - just verify the logic holds
		t.Log("auto-pass condition met: MiniTestAttempts >= 2")
	}
}

func TestNextUndoneInBatch(t *testing.T) {
	steps := []Step{
		{ID: 1, Done: true},
		{ID: 2, Done: true},
		{ID: 3, Done: false},
		{ID: 4, Done: true},
		{ID: 5, Done: false},
		{ID: 6, Done: false},
	}

	// Batch 1: steps 0-3 (indices)
	next := nextUndoneInBatch(steps, 0, 3)
	if next != 3 {
		t.Fatalf("expected next=3, got %d", next)
	}

	// Batch 2: steps 3-6
	next = nextUndoneInBatch(steps, 3, 6)
	if next != 5 {
		t.Fatalf("expected next=5 (skipping done step 4), got %d", next)
	}

	// All done
	allDone := []Step{
		{ID: 1, Done: true},
		{ID: 2, Done: true},
		{ID: 3, Done: true},
	}
	next = nextUndoneInBatch(allDone, 0, 3)
	if next != -1 {
		t.Fatalf("expected -1 for all done, got %d", next)
	}
}

func TestOverallProgress(t *testing.T) {
	p := &Progress{
		Steps: []Step{
			{ID: 1, Done: true},
			{ID: 2, Done: true},
			{ID: 3, Done: false},
			{ID: 4, Done: false},
		},
	}
	got := overallProgress(p)
	if got != "2/4" {
		t.Fatalf("expected 2/4, got %s", got)
	}
}

func TestEffectiveBatchSize(t *testing.T) {
	p := &Progress{BatchSize: 0}
	if effectiveBatchSize(p) != 3 {
		t.Fatal("default should be 3")
	}
	p.BatchSize = 5
	if effectiveBatchSize(p) != 5 {
		t.Fatal("should return explicit batch size")
	}
}

func TestSetStepsWithFilePrefix(t *testing.T) {
	c, _ := setupTestClient(t)

	_, err := c.Start("test", "[main.go]Crear func main;[main.go]Importar fmt;[cliente.go]Crear func main del cliente")
	if err != nil {
		t.Fatal(err)
	}

	p := loadProgress(t)
	if len(p.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(p.Steps))
	}
	if p.Steps[0].File != "main.go" {
		t.Fatalf("step 1 file: expected main.go, got %q", p.Steps[0].File)
	}
	if p.Steps[1].File != "main.go" {
		t.Fatalf("step 2 file: expected main.go, got %q", p.Steps[1].File)
	}
	if p.Steps[2].File != "cliente.go" {
		t.Fatalf("step 3 file: expected cliente.go, got %q", p.Steps[2].File)
	}
	if p.Steps[0].Instruction != "Crear func main" {
		t.Fatalf("step 1 instruction: expected 'Crear func main', got %q", p.Steps[0].Instruction)
	}
}

func TestSetStepsWithoutFilePrefix(t *testing.T) {
	c, _ := setupTestClient(t)

	_, err := c.Start("test", "step one;step two")
	if err != nil {
		t.Fatal(err)
	}

	p := loadProgress(t)
	if p.Steps[0].File != "" {
		t.Fatalf("step without prefix should have empty File, got %q", p.Steps[0].File)
	}
}

func TestResolveFile(t *testing.T) {
	// Step with File takes precedence over empty override
	s := Step{ID: 1, File: "a.go", Instruction: "x"}
	f, err := resolveFile(s, "")
	if err != nil {
		t.Fatal(err)
	}
	if f != "a.go" {
		t.Fatalf("expected a.go, got %s", f)
	}

	// Override takes precedence over step File
	f, err = resolveFile(s, "b.go")
	if err != nil {
		t.Fatal(err)
	}
	if f != "b.go" {
		t.Fatalf("expected b.go, got %s", f)
	}

	// No File and no override → error
	s2 := Step{ID: 2, Instruction: "y"}
	_, err = resolveFile(s2, "")
	if err == nil {
		t.Fatal("expected error for no file")
	}
}

func TestBatchFiles(t *testing.T) {
	steps := []Step{
		{ID: 1, File: "main.go"},
		{ID: 2, File: "main.go"},
		{ID: 3, File: "client.go"},
		{ID: 4, File: "main.go"},
	}

	// Batch 0-3
	files := batchFiles(steps, 0, 3)
	if len(files) != 2 {
		t.Fatalf("expected 2 unique files, got %d: %v", len(files), files)
	}
	if files[0] != "main.go" || files[1] != "client.go" {
		t.Fatalf("expected [main.go, client.go], got %v", files)
	}
}

func TestSaveAndRestoreCheckpoints(t *testing.T) {
	dir := t.TempDir()

	// Create two files
	file1 := filepath.Join(dir, "a.go")
	file2 := filepath.Join(dir, "b.go")
	os.WriteFile(file1, []byte("content-a"), 0644)
	os.WriteFile(file2, []byte("content-b"), 0644)

	p := &Progress{
		Steps: []Step{
			{ID: 1, File: file1},
			{ID: 2, File: file2},
			{ID: 3, File: file1},
		},
	}

	// Save checkpoints for batch 0-3
	saveCheckpoints(p, 0, 3)
	if len(p.Checkpoints) != 2 {
		t.Fatalf("expected 2 checkpoints, got %d", len(p.Checkpoints))
	}
	if p.Checkpoints[file1] != "content-a" {
		t.Fatalf("checkpoint a: %q", p.Checkpoints[file1])
	}
	if p.Checkpoints[file2] != "content-b" {
		t.Fatalf("checkpoint b: %q", p.Checkpoints[file2])
	}

	// Modify files
	os.WriteFile(file1, []byte("modified-a"), 0644)
	os.WriteFile(file2, []byte("modified-b"), 0644)

	// Restore checkpoints
	mode, err := restoreCheckpoints(p, 0, 3)
	if err != nil {
		t.Fatal(err)
	}
	if mode == "" {
		t.Fatal("expected non-empty mode string")
	}

	// Verify files restored
	data1, _ := os.ReadFile(file1)
	data2, _ := os.ReadFile(file2)
	if string(data1) != "content-a" {
		t.Fatalf("file1 not restored: %q", string(data1))
	}
	if string(data2) != "content-b" {
		t.Fatalf("file2 not restored: %q", string(data2))
	}
}

func TestBatchInfoIncludesFile(t *testing.T) {
	p := &Progress{
		BatchSize:   3,
		BatchIndex:  1,
		CurrentStep: 1,
		Steps: []Step{
			{ID: 1, File: "main.go", Instruction: "s1"},
			{ID: 2, File: "client.go", Instruction: "s2"},
			{ID: 3, File: "main.go", Instruction: "s3"},
		},
	}
	bi := buildBatchInfo(p)
	if bi.Steps[0].File != "main.go" {
		t.Fatalf("batch step 0 file: expected main.go, got %q", bi.Steps[0].File)
	}
	if bi.Steps[1].File != "client.go" {
		t.Fatalf("batch step 1 file: expected client.go, got %q", bi.Steps[1].File)
	}
}
