// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package platform

type Model struct {
	Component     string         `json:"component"`
	Description   string         `json:"description"`
	Variants      []Variant      `json:"variants"`
	Pieces        []Piece        `json:"pieces"`
	Units         []Unit         `json:"units"`
	Queries       []AccessQuery  `json:"queries"`
	Findings      []Finding      `json:"findings"`
	ProposedEdits []ProposedEdit `json:"proposedEdits"`
}

type Variant struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
}

type Piece struct {
	Name   string   `json:"name"`
	Source string   `json:"source"`
	Units  []string `json:"units"`
}

type Unit struct {
	Unit      string            `json:"unit"`
	Variant   string            `json:"variant"`
	Piece     string            `json:"piece"`
	Kind      string            `json:"kind"`
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels,omitempty"`
	Rules     []PolicyRule      `json:"rules,omitempty"`
	Subjects  []Subject         `json:"subjects,omitempty"`
	RoleRef   *RoleRef          `json:"roleRef,omitempty"`
	Intent    string            `json:"intent,omitempty"`
}

type PolicyRule struct {
	APIGroups     []string `json:"apiGroups"`
	Resources     []string `json:"resources"`
	Verbs         []string `json:"verbs"`
	ResourceNames []string `json:"resourceNames,omitempty"`
}

type Subject struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type RoleRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type AccessQuery struct {
	Name      string  `json:"name"`
	Verb      string  `json:"verb"`
	Resource  string  `json:"resource"`
	Namespace string  `json:"namespace"`
	Grants    []Grant `json:"grants"`
}

type Grant struct {
	Subject     string `json:"subject"`
	Via         string `json:"via"`
	SourcePiece string `json:"sourcePiece"`
	Scope       string `json:"scope"`
}

type Finding struct {
	ID             string `json:"id"`
	Severity       string `json:"severity"`
	Variant        string `json:"variant"`
	Piece          string `json:"piece"`
	Reason         string `json:"reason"`
	Recommendation string `json:"recommendation"`
	BetterPlace    string `json:"betterPlace"`
}

type ProposedEdit struct {
	ID          string     `json:"id"`
	Mode        string     `json:"mode"`
	TargetUnit  string     `json:"targetUnit"`
	Variant     string     `json:"variant"`
	Diff        []EditDiff `json:"diff"`
	HumanReview string     `json:"humanReview"`
}

type EditDiff struct {
	Path string `json:"path"`
	From any    `json:"from"`
	To   any    `json:"to"`
}
