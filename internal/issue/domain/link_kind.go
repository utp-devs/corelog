package domain

// linkKindLabels holds the wording of a link kind seen from the source side (outward) and from the target side
// (inward). Relates reads the same from both sides.
var linkKindLabels = map[IssueLinkKind][2]string{
	IssueLinkBlocks:     {"bloquea", "bloqueada por"},
	IssueLinkRelates:    {"se relaciona con", "se relaciona con"},
	IssueLinkDuplicates: {"duplica a", "es duplicada por"},
	IssueLinkClones:     {"clona a", "es clonada por"},
	IssueLinkCauses:     {"causa", "es causada por"},
	IssueLinkImplements: {"implementa", "es implementada por"},
}

// IssueLinkKinds lists every link kind in display order.
var IssueLinkKinds = []IssueLinkKind{
	IssueLinkBlocks, IssueLinkRelates, IssueLinkDuplicates, IssueLinkClones, IssueLinkCauses, IssueLinkImplements,
}

// Valid reports whether the kind is one of the supported link kinds.
func (k IssueLinkKind) Valid() bool {
	_, ok := linkKindLabels[k]
	return ok
}

// Label returns the Spanish wording of the kind for the given side of a link.
func (k IssueLinkKind) Label(outward bool) string {
	labels, ok := linkKindLabels[k]
	if !ok {
		return ""
	}
	if outward {
		return labels[0]
	}
	return labels[1]
}

// ForbidsCycles reports whether links of this kind must not form a cycle. Only blocking chains are ordered.
func (k IssueLinkKind) ForbidsCycles() bool {
	return k == IssueLinkBlocks
}

// ParseIssueLinkKind converts the wire value of a kind. Unknown values are rejected.
func ParseIssueLinkKind(value string) (IssueLinkKind, error) {
	kind := IssueLinkKind(value)
	if !kind.Valid() {
		return "", ErrUnknownLinkKind
	}
	return kind, nil
}
