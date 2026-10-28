import { type Component, createUniqueId } from 'solid-js'

export interface SubrefineLogoProps {
  class?: string
  size?: number
  pulsing?: boolean
}

/**
 * Brand mark: three inbound subscription streams converging through a refine
 * stage into a single outbound profile. Gradient stops reuse the theme tokens,
 * so the mark follows the active accent colour.
 */
export const SubrefineLogo: Component<SubrefineLogoProps> = props => {
  const uid = createUniqueId()
  const liquidId = `subrefine-liquid-${uid}`
  const specularId = `subrefine-specular-${uid}`
  const coreId = `subrefine-core-${uid}`

  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 32 32"
      fill="none"
      class={props.class}
      width={props.size}
      height={props.size}
      aria-label="subrefine"
    >
      <defs>
        <linearGradient id={liquidId} x1="10%" y1="0%" x2="90%" y2="100%">
          <stop offset="0%" stop-color="var(--brand-mint)" />
          <stop offset="35%" stop-color="var(--brand-cyan)" />
          <stop offset="70%" stop-color="var(--brand-indigo)" />
          <stop offset="100%" stop-color="var(--brand-violet)" />
        </linearGradient>

        <linearGradient id={specularId} x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stop-color="#ffffff" stop-opacity="0.9" />
          <stop offset="40%" stop-color="#ffffff" stop-opacity="0.25" />
          <stop offset="100%" stop-color="#ffffff" stop-opacity="0" />
        </linearGradient>

        <linearGradient id={coreId} x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stop-color="#ffffff" />
          <stop offset="60%" stop-color="#38bdf8" />
          <stop offset="100%" stop-color="#0284c7" />
        </linearGradient>
      </defs>

      {/* Aperture body */}
      <path
        d="M 27.69 9.25 L 16 2.5 L 4.31 9.25 L 4.31 22.75 L 16 29.5 L 27.69 22.75 L 27.69 17.5 L 21.5 17.5 L 21.5 14.5 L 27.69 14.5 Z"
        fill={`url(#${liquidId})`}
      />

      {/* Top specular bevel */}
      <path
        d="M 4.31 9.25 L 16 2.5 L 27.69 9.25"
        stroke={`url(#${specularId})`}
        stroke-width="0.8"
        stroke-linecap="round"
        stroke-linejoin="round"
      />

      {/* Inbound streams converging on the refine core */}
      <path
        d="M 8.5 11.5 L 14 16 L 8.5 20.5"
        stroke={`url(#${coreId})`}
        stroke-width="1.9"
        stroke-linecap="round"
        stroke-linejoin="round"
        opacity={props.pulsing ? 0.95 : 0.85}
      />
      <circle cx="16.4" cy="16" r="2.35" fill={`url(#${coreId})`} />
    </svg>
  )
}
