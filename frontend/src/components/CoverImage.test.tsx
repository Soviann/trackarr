import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent } from '@testing-library/preact'
import { CoverImage } from './CoverImage'

describe('CoverImage', () => {
  it('renders an img tag with the provided className when coverUrl is valid', () => {
    const { container } = render(
      <CoverImage
        coverUrl="https://example.com/poster.jpg"
        type="movie"
        alt="Test Movie"
        className="custom-cover-class"
      />
    )
    const img = container.querySelector('img')
    expect(img).not.toBeNull()
    expect(img?.getAttribute('src')).toBe('https://example.com/poster.jpg')
    expect(img?.getAttribute('alt')).toBe('Test Movie')
    expect(img?.classList.contains('custom-cover-class')).toBe(true)
  })

  it('renders placeholder with className and without inline position:relative when coverUrl is null', () => {
    const { container } = render(
      <CoverImage
        coverUrl={null}
        type="movie"
        className="custom-cover-class"
      />
    )
    const element = container.querySelector('.custom-cover-class') as HTMLElement
    expect(element).not.toBeNull()
    // Should not override caller's class positioning with inline position:relative
    expect(element.style.position).not.toBe('relative')
  })

  it('renders placeholder with className when image load fails', () => {
    const { container } = render(
      <CoverImage
        coverUrl="https://example.com/broken.jpg"
        type="series"
        className="custom-cover-class"
      />
    )
    const img = container.querySelector('img')
    expect(img).not.toBeNull()

    // Trigger image error
    fireEvent.error(img!)

    const element = container.querySelector('.custom-cover-class') as HTMLElement
    expect(element).not.toBeNull()
    expect(element.tagName.toLowerCase()).not.toBe('img')
    expect(element.style.position).not.toBe('relative')
  })

  it('forwards onClick to the placeholder element', () => {
    const handleClick = vi.fn()
    const { container } = render(
      <CoverImage
        coverUrl={null}
        type="movie"
        className="custom-cover-class"
        onClick={handleClick}
      />
    )
    const element = container.querySelector('.custom-cover-class') as HTMLElement
    expect(element).not.toBeNull()
    fireEvent.click(element)
    expect(handleClick).toHaveBeenCalledTimes(1)
  })
})
