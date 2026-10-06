package group

import (
	"reflect"
	"testing"
)

func TestMarkMemberAsDisqualified(t *testing.T) {
	var tests = map[string]struct {
		initialMembers              []MemberIndex
		updateFunc                  func(g *Group)
		expectedDisqualifiedMembers []MemberIndex
		expectedInactiveMembers     []MemberIndex
	}{
		"mark member as disqualified": {
			initialMembers: []MemberIndex{19, 11, 31, 33},
			updateFunc: func(g *Group) {
				g.MarkMemberAsDisqualified(19)
			},
			expectedDisqualifiedMembers: []MemberIndex{19},
			expectedInactiveMembers:     []MemberIndex{},
		},
		"mark member as disqualified twice": {
			initialMembers: []MemberIndex{19, 11, 31, 33},
			updateFunc: func(g *Group) {
				g.MarkMemberAsDisqualified(11)
				g.MarkMemberAsDisqualified(11)
			},
			expectedDisqualifiedMembers: []MemberIndex{11},
			expectedInactiveMembers:     []MemberIndex{},
		},
		"mark member from out of the group as disqualified": {
			initialMembers: []MemberIndex{19, 11, 31, 33},
			updateFunc: func(g *Group) {
				g.MarkMemberAsDisqualified(88)
			},
			expectedDisqualifiedMembers: []MemberIndex{},
			expectedInactiveMembers:     []MemberIndex{},
		},
		"mark all members as disqualified": {
			initialMembers: []MemberIndex{11, 12, 13},
			updateFunc: func(g *Group) {
				g.MarkMemberAsDisqualified(11)
				g.MarkMemberAsDisqualified(13)
				g.MarkMemberAsDisqualified(12)
			},
			expectedDisqualifiedMembers: []MemberIndex{11, 13, 12},
			expectedInactiveMembers:     []MemberIndex{},
		},
		"mark member as inactive": {
			initialMembers: []MemberIndex{19, 11, 31, 33},
			updateFunc: func(g *Group) {
				g.MarkMemberAsInactive(31)
			},
			expectedDisqualifiedMembers: []MemberIndex{},
			expectedInactiveMembers:     []MemberIndex{31},
		},
		"mark member as inactive twice": {
			initialMembers: []MemberIndex{19, 11, 31, 33},
			updateFunc: func(g *Group) {
				g.MarkMemberAsInactive(33)
				g.MarkMemberAsInactive(33)
			},
			expectedDisqualifiedMembers: []MemberIndex{},
			expectedInactiveMembers:     []MemberIndex{33},
		},
		"mark member from out of the group as inactive": {
			initialMembers: []MemberIndex{19, 11, 31, 33},
			updateFunc: func(g *Group) {
				g.MarkMemberAsInactive(99)
			},
			expectedDisqualifiedMembers: []MemberIndex{},
			expectedInactiveMembers:     []MemberIndex{},
		},
		"mark all members as inactive": {
			initialMembers: []MemberIndex{19, 18, 17, 16},
			updateFunc: func(g *Group) {
				g.MarkMemberAsInactive(17)
				g.MarkMemberAsInactive(19)
				g.MarkMemberAsInactive(16)
				g.MarkMemberAsInactive(18)
			},
			expectedDisqualifiedMembers: []MemberIndex{},
			expectedInactiveMembers:     []MemberIndex{17, 19, 16, 18},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			group := &Group{
				memberIndexes:             test.initialMembers,
				disqualifiedMemberIndexes: []MemberIndex{},
				inactiveMemberIndexes:     []MemberIndex{},
			}

			if test.updateFunc != nil {
				test.updateFunc(group)
			}

			if !reflect.DeepEqual(
				test.expectedDisqualifiedMembers,
				group.disqualifiedMemberIndexes,
			) {
				t.Fatalf(
					"unexpected list of disqualified members\nexpected: %v\nactual:   %v\n",
					test.expectedDisqualifiedMembers,
					group.disqualifiedMemberIndexes,
				)
			}

			if !reflect.DeepEqual(
				test.expectedInactiveMembers,
				group.inactiveMemberIndexes,
			) {
				t.Fatalf(
					"unexpected list of inactive members\nexpected: %v\nactual:   %v\n",
					test.expectedInactiveMembers,
					group.inactiveMemberIndexes,
				)
			}
		})
	}
}

func TestIsDisqualified(t *testing.T) {
	group := &Group{
		memberIndexes: []MemberIndex{19, 11, 31, 33},
	}

	if group.isDisqualified(19) {
		t.Errorf("member should not be disqualified at this point")
	}

	group.MarkMemberAsDisqualified(19)

	if !group.isDisqualified(19) {
		t.Errorf("member should be disqualified at this point")
	}
}

func TestIsInactive(t *testing.T) {
	group := &Group{
		memberIndexes: []MemberIndex{19, 11, 31, 33},
	}

	if group.isInactive(31) {
		t.Errorf("member should ne be inactive at this point")
	}

	group.MarkMemberAsInactive(31)

	if !group.isInactive(31) {
		t.Errorf("member should be inactive at this point")
	}
}

func TestOperatingMembers(t *testing.T) {
	var tests = map[string]struct {
		initialMembers           []MemberIndex
		updateFunc               func(g *Group)
		expectedOperatingMembers []MemberIndex
	}{
		"all members remain operating": {
			initialMembers:           []MemberIndex{10, 12, 33, 11},
			expectedOperatingMembers: []MemberIndex{10, 12, 33, 11},
		},
		"one member disqualified": {
			initialMembers: []MemberIndex{99, 98, 12, 33, 44},
			updateFunc: func(g *Group) {
				g.MarkMemberAsDisqualified(98)
			},
			expectedOperatingMembers: []MemberIndex{99, 12, 33, 44},
		},
		"one member inactive": {
			initialMembers: []MemberIndex{38, 19, 39, 22, 11},
			updateFunc: func(g *Group) {
				g.MarkMemberAsInactive(11)
			},
			expectedOperatingMembers: []MemberIndex{38, 19, 39, 22},
		},
		"one member disqualified and one member inactive": {
			initialMembers: []MemberIndex{19, 11, 31, 33},
			updateFunc: func(g *Group) {
				g.MarkMemberAsDisqualified(19)
				g.MarkMemberAsInactive(33)
			},
			expectedOperatingMembers: []MemberIndex{11, 31},
		},
		"all but one inactive": {
			initialMembers: []MemberIndex{28, 19, 29},
			updateFunc: func(g *Group) {
				g.MarkMemberAsInactive(19)
				g.MarkMemberAsInactive(29)
			},
			expectedOperatingMembers: []MemberIndex{28},
		},
		"all but one disqualified": {
			initialMembers: []MemberIndex{92, 11, 20},
			updateFunc: func(g *Group) {
				g.MarkMemberAsDisqualified(92)
				g.MarkMemberAsDisqualified(11)
			},
			expectedOperatingMembers: []MemberIndex{20},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			group := &Group{}
			group.memberIndexes = test.initialMembers

			if test.updateFunc != nil {
				test.updateFunc(group)
			}

			operatingMembers := group.OperatingMemberIndexes()
			if !reflect.DeepEqual(
				test.expectedOperatingMembers,
				operatingMembers,
			) {
				t.Fatalf(
					"unexpected list of operating members\nexpected: %v\nactual:   %v\n",
					test.expectedOperatingMembers,
					operatingMembers,
				)
			}

		})
	}
}
