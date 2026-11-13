package pad

import (
	"errors"
	"strings"

	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/utils"
)

// RestoreRevision appends a revision that makes the pad's content what it
// was at rev: text, formatting and authorship. Runs without an author (text
// stored before every insert had to carry one) are attributed to authorId,
// or to the system author when no author is given.
func (p *Pad) RestoreRevision(rev int, authorId *string) (*int, error) {
	if rev < 0 || rev > p.Head {
		return nil, errors.New("revision out of range")
	}
	target := p.GetInternalRevisionAText(rev)
	if target == nil {
		return nil, errors.New("could not reconstruct the revision")
	}
	fallbackAuthor := SystemAuthorId
	if authorId != nil && *authorId != "" {
		fallbackAuthor = *authorId
	}

	oldText := p.Text()
	builder := changeset.NewBuilder(utils.RuneCount(oldText))

	// Insert the old content before the pad's final newline, run by run
	// with the run's attributes. The target's own final newline stays.
	ops, err := changeset.DeserializeOps(target.Attribs)
	if err != nil {
		return nil, err
	}
	bodyLen := utils.RuneCount(target.Text) - 1
	pos := 0
	for _, op := range *ops {
		start, end := pos, min(pos+op.Chars, bodyLen)
		pos += op.Chars
		if start >= end {
			continue
		}
		attribs := changeset.FromString(op.Attribs, &p.Pool)
		if v := attribs.Get("author"); v == nil || *v == "" {
			attribs.Set("author", fallbackAuthor)
		}
		builder.Insert(utils.RuneSlice(target.Text, start, end), changeset.AttribsString(attribs.String()), nil)
	}

	// Remove the current content except the final newline.
	oldBody := strings.TrimSuffix(oldText, "\n")
	if lastNL := strings.LastIndex(oldBody, "\n"); lastNL >= 0 {
		builder.Remove(utils.RuneCount(oldBody[:lastNL+1]), strings.Count(oldBody, "\n"))
		builder.Remove(utils.RuneCount(oldBody[lastNL+1:]), 0)
	} else {
		builder.Remove(utils.RuneCount(oldBody), 0)
	}

	revisionAuthor := authorId
	if revisionAuthor == nil || *revisionAuthor == "" {
		sys := SystemAuthorId
		revisionAuthor = &sys
	}
	return p.AppendRevision(builder.ToString(), revisionAuthor)
}
