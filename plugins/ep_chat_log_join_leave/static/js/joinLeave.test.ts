import {describe, expect, it} from 'vitest'
import {joinLeaveOf, pickName} from './joinLeave'

describe('ep_chat_log_join_leave', () => {
  const me = 'a.me'

  it('ignores ordinary chat messages', () => {
    expect(joinLeaveOf({text: 'hi'}, 'a.anna', false, me)).toBeNull()
    expect(joinLeaveOf(null, 'a.anna', false, me)).toBeNull()
    expect(joinLeaveOf({ep_chat_log_join_leave: 'other'}, 'a.anna', false, me)).toBeNull()
  })

  it('a toast when someone else joins or leaves', () => {
    expect(joinLeaveOf({ep_chat_log_join_leave: 'join'}, 'a.anna', false, me)).toEqual({type: 'join', toast: true})
    expect(joinLeaveOf({ep_chat_log_join_leave: 'leave'}, 'a.anna', false, me)).toEqual({type: 'leave', toast: true})
  })

  it('no toast about oneself', () => {
    expect(joinLeaveOf({ep_chat_log_join_leave: 'join'}, me, false, me)).toEqual({type: 'join', toast: false})
  })

  it('no toast for a message from the history', () => {
    expect(joinLeaveOf({ep_chat_log_join_leave: 'join'}, 'a.anna', true, me)).toEqual({type: 'join', toast: false})
  })

  it('finds the best name', () => {
    const names = new Map([['a.anna', 'Anna']])
    expect(pickName('a.anna', 'Unnamed', names, {'a.anna': {name: 'Old'}})).toBe('Anna')
    expect(pickName('a.ben', 'Unnamed', names, {'a.ben': {name: 'Ben'}})).toBe('Ben')
    expect(pickName('a.carl', 'Unnamed', names, {'a.carl': {name: null}})).toBe('Unnamed')
    expect(pickName('a.dora', '', names, null)).toBe('a.dora')
  })
})
