package mqtt

import (
	"fmt"
	"strings"
)

func FormatDigest(msgs []Message, maxChunkLen int, maxPosts int) []string {
	if len(msgs) == 0 {
		return nil
	}

	var lines []string
	for _, m := range msgs {
		line := fmt.Sprintf("`%s` %s — %s", m.Timestamp.Format("15:04:05"), m.Topic, m.Payload)
		lines = append(lines, line)
	}

	var chunks []string
	var current strings.Builder

	for i, line := range lines {
		wouldExceedChunkLen := current.Len() > 0 && current.Len()+len(line)+1 > maxChunkLen
		atLastPost := len(chunks) == maxPosts-1

		if wouldExceedChunkLen && !atLastPost {
			chunks = append(chunks, current.String())
			current.Reset()
		}

		if wouldExceedChunkLen && atLastPost {
			suppressed := len(lines) - i
			current.WriteString(fmt.Sprintf("\n…+%d more messages suppressed", suppressed))
			chunks = append(chunks, current.String())
			return chunks
		}

		if current.Len() > 0 {
			current.WriteByte('\n')
		}
		current.WriteString(line)
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}

	if len(chunks) > maxPosts {
		kept := chunks[:maxPosts-1]
		suppressed := 0
		for _, c := range chunks[maxPosts-1:] {
			suppressed += strings.Count(c, "\n") + 1
		}
		lastChunk := chunks[maxPosts-1]
		remainingInLast := strings.Count(lastChunk, "\n") + 1
		notice := fmt.Sprintf("%s\n…+%d more messages suppressed", lastChunk, suppressed-remainingInLast)
		chunks = append(kept, notice)
	}

	return chunks
}
