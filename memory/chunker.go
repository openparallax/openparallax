package memory

import "strings"

// Chunk is a segment of text from a document.
type Chunk struct {
	Text      string
	StartLine int
	EndLine   int
}

// ChunkMarkdown splits markdown text into overlapping chunks.
// targetTokens is the approximate size of each chunk in tokens (~4 chars/token).
// overlap is the number of tokens of overlap between consecutive chunks.
func ChunkMarkdown(text string, targetTokens, overlap int) []Chunk {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	lines := strings.Split(text, "\n")

	targetChars := targetTokens * 4
	overlapChars := overlap * 4

	var chunks []Chunk
	var current strings.Builder
	startLine := 1
	currentChars := 0

	for i, line := range lines {
		lineNum := i + 1
		lineLen := len(line) + 1 // +1 for newline

		if currentChars+lineLen > targetChars && currentChars > 0 {
			chunks = append(chunks, Chunk{
				Text:      strings.TrimSpace(current.String()),
				StartLine: startLine,
				EndLine:   lineNum - 1,
			})

			// Overlap: keep the last overlapChars of the current chunk.
			text := current.String()
			current.Reset()
			if overlapChars > 0 && len(text) > overlapChars {
				overlap := text[len(text)-overlapChars:]
				current.WriteString(overlap)
				currentChars = len(overlap)
				// Approximate the start line for the overlap.
				overlapLines := strings.Count(overlap, "\n")
				startLine = lineNum - overlapLines
			} else {
				currentChars = 0
				startLine = lineNum
			}
		}

		current.WriteString(line)
		current.WriteString("\n")
		currentChars += lineLen
	}

	// Flush remaining content.
	if currentChars > 0 {
		chunks = append(chunks, Chunk{
			Text:      strings.TrimSpace(current.String()),
			StartLine: startLine,
			EndLine:   len(lines),
		})
	}

	return chunks
}
