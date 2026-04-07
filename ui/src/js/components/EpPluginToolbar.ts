/**
 * EpPluginToolbar — Web Component for plugin-contributed toolbar buttons/selects.
 *
 * Renders directly into the light DOM (no Shadow DOM) so it inherits the pad's
 * CSS styles.  Listens on the EventBus for `custom:toolbar:register:button` and
 * `custom:toolbar:register:select` events and re-renders whenever a plugin
 * registers a new item.
 */

import { editorBus } from '../core/EventBus'

interface ButtonConfig {
  key: string
  title: string
  icon: string
  onClick: () => void
}

interface SelectConfig {
  key: string
  title: string
  options: { label: string; value: string }[]
  onChange: (value: string) => void
}

class EpPluginToolbar extends HTMLElement {
  private buttons: Map<string, ButtonConfig> = new Map()
  private selects: Map<string, SelectConfig> = new Map()
  private unsubs: Array<() => void> = []

  connectedCallback() {
    this.style.display = 'contents'

    // Listen for plugin toolbar registrations via EventBus
    this.unsubs.push(editorBus.on('custom:toolbar:register:button' as any, (config: ButtonConfig) => {
      this.buttons.set(config.key, config)
      this.render()
    }))
    this.unsubs.push(editorBus.on('custom:toolbar:register:select' as any, (config: SelectConfig) => {
      this.selects.set(config.key, config)
      this.render()
    }))
    this.render()
  }

  disconnectedCallback() {
    for (const unsub of this.unsubs) unsub()
    this.unsubs = []
  }

  private render() {
    // Clear and re-render all plugin toolbar items
    this.innerHTML = ''

    for (const [key, config] of this.buttons) {
      const li = document.createElement('li')
      li.dataset.key = key
      li.dataset.plugin = 'true'
      const a = document.createElement('a')
      a.title = config.title
      a.addEventListener('click', (e) => {
        e.preventDefault()
        config.onClick()
        editorBus.emit('toolbar:button:click', { key })
      })
      const span = document.createElement('span')
      span.className = `buttonicon ${config.icon}`
      a.appendChild(span)
      li.appendChild(a)
      this.appendChild(li)
    }

    for (const [key, config] of this.selects) {
      const li = document.createElement('li')
      li.dataset.key = key
      li.dataset.plugin = 'true'
      const dropdown = document.createElement('ep-dropdown')
      dropdown.setAttribute('align', 'left')
      dropdown.setAttribute('trigger', 'click')
      dropdown.className = 'plugin-select'

      const trigger = document.createElement('button')
      trigger.type = 'button'
      trigger.slot = 'trigger'
      trigger.className = 'editbarbutton plugin-select-trigger'
      trigger.title = config.title
      trigger.textContent = config.title

      const content = document.createElement('div')
      content.slot = 'content'
      for (const opt of config.options) {
        const item = document.createElement('ep-dropdown-item')
        item.setAttribute('value', opt.value)
        item.textContent = opt.label
        content.appendChild(item)
      }

      dropdown.addEventListener('ep-dropdown-select', ((event: CustomEvent) => {
        config.onChange(event.detail.value)
      }) as EventListener)

      dropdown.append(trigger, content)
      li.appendChild(dropdown)
      trigger.addEventListener('click', (event) => {
        event.stopPropagation()
      })
      this.appendChild(li)
    }
  }
}

customElements.define('ep-plugin-toolbar', EpPluginToolbar)
export { EpPluginToolbar }
