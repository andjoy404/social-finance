import { render, screen, fireEvent } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { RowActionMenu } from './RowActionMenu'

describe('RowActionMenu', () => {
  it('is initially closed', () => {
    render(
      <RowActionMenu items={[{ label: 'Edit', onClick: vi.fn() }]} />
    )
    expect(screen.getByRole('button', { name: /aksi/i })).toBeInTheDocument()
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })

  it('trigger has aria-label Aksi and icon', () => {
    render(
      <RowActionMenu items={[{ label: 'Edit', onClick: vi.fn() }]} />
    )
    const trigger = screen.getByRole('button', { name: /aksi/i })
    expect(trigger).toHaveAttribute('title', 'Aksi')
    const icon = trigger.querySelector('svg')
    expect(icon).toBeInTheDocument()
  })

  it('opens on click', () => {
    render(
      <RowActionMenu items={[{ label: 'Edit', onClick: vi.fn() }]} />
    )
    const trigger = screen.getByRole('button', { name: /aksi/i })
    fireEvent.click(trigger)
    expect(screen.queryByRole('menu')).toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: /edit/i })).toBeInTheDocument()
  })

  it('sets aria-expanded to true when open', () => {
    render(
      <RowActionMenu items={[{ label: 'Edit', onClick: vi.fn() }]} />
    )
    const trigger = screen.getByRole('button', { name: /aksi/i })
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    fireEvent.click(trigger)
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
  })

  it('closes on second click', () => {
    render(
      <RowActionMenu items={[{ label: 'Edit', onClick: vi.fn() }]} />
    )
    const trigger = screen.getByRole('button', { name: /aksi/i })
    fireEvent.click(trigger)
    expect(screen.queryByRole('menu')).toBeInTheDocument()
    fireEvent.click(trigger)
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })

  it('closes on outside click', () => {
    render(
      <>
        <button data-testid="outside-button" type="button">
          Other
        </button>
        <RowActionMenu items={[{ label: 'Edit', onClick: vi.fn() }]} />
      </>
    )
    const trigger = screen.getByRole('button', { name: /aksi/i })
    fireEvent.click(trigger)
    expect(screen.queryByRole('menu')).toBeInTheDocument()
    fireEvent.mouseDown(screen.getByTestId('outside-button'))
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })

  it('closes on Escape key', () => {
    render(
      <RowActionMenu items={[{ label: 'Edit', onClick: vi.fn() }]} />
    )
    const trigger = screen.getByRole('button', { name: /aksi/i })
    fireEvent.click(trigger)
    expect(screen.queryByRole('menu')).toBeInTheDocument()
    fireEvent.keyDown(document.body, { key: 'Escape' })
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })

  it('Edit callback fires exactly once on click', () => {
    const onClick = vi.fn()
    render(
      <RowActionMenu items={[{ label: 'Edit', onClick }]} />
    )
    const trigger = screen.getByRole('button', { name: /aksi/i })
    fireEvent.click(trigger)
    fireEvent.click(screen.getByRole('menuitem', { name: /edit/i }))
    expect(onClick).toHaveBeenCalledTimes(1)
  })

  it('Edit closes the menu', () => {
    render(
      <RowActionMenu items={[{ label: 'Edit', onClick: vi.fn() }]} />
    )
    const trigger = screen.getByRole('button', { name: /aksi/i })
    fireEvent.click(trigger)
    expect(screen.queryByRole('menu')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('menuitem', { name: /edit/i }))
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })

  it('renders the menu through a portal', () => {
    render(
      <RowActionMenu items={[{ label: 'Edit', onClick: vi.fn() }]} />
    )
    const trigger = screen.getByRole('button', { name: /aksi/i })
    const wrapper = trigger.closest('.sf-row-action')
    fireEvent.click(trigger)
    const menu = screen.getByRole('menu')
    expect(menu).toHaveAttribute('role', 'menu')
    if (wrapper) {
      expect(wrapper.contains(menu)).toBe(false)
    }
  })
})
