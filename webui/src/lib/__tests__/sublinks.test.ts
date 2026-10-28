import { describe, expect, it } from 'vitest'
import { DELIVERY_OPTIONS, deliveryUrl } from '../sublinks'

describe('deliveryUrl', () => {
  it('encodes tokens that carry URL-active characters', () => {
    const url = deliveryUrl('hk', 'a+b/c?d=e', 'sing-box')
    expect(url).toBe('http://localhost:3000/s/hk?token=a%2Bb%2Fc%3Fd%3De&target=sing-box')
  })

  it('percent-escapes an id that could otherwise add path segments', () => {
    expect(deliveryUrl('a/b', 't', 'clash')).toContain('/s/a%2Fb?')
  })

  it('offers exactly the three targets the server negotiates', () => {
    expect(DELIVERY_OPTIONS.map(o => o.target)).toEqual(['sing-box', 'clash', 'uri'])
  })
})
