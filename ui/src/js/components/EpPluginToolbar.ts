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

  connectedCallback() {
    // Listen for plugin toolbar registrations via EventBus
    editorBus.on('custom:toolbar:register:button' as any, (config: ButtonConfig) => {
      this.buttons.set(config.key, config)
      this.render()
    })
    editorBus.on('custom:toolbar:register:select' as any, (config: SelectConfig) => {
      this.selects.set(config.key, config)
      this.render()
    })
    this.render()
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
      const select = document.createElement('select')
      select.className = 'plugin-select'
      select.title = config.title

      // Add dummy/title option
      const titleOpt = document.createElement('option')
      titleOpt.value = ''
      titleOpt.textContent = config.title
      titleOpt.selected = true
      titleOpt.disabled = true
      select.appendChild(titleOpt)

      for (const opt of config.options) {
        const option = document.createElement('option')
        option.value = opt.value
        option.textContent = opt.label
        select.appendChild(option)
      }

      select.addEventListener('change', () => {
        config.onChange(select.value)
        select.value = '' // reset to title
      })

      li.appendChild(select)
      this.appendChild(li)
    }
  }
}

customElements.define('ep-plugin-toolbar', EpPluginToolbar)
export { EpPluginToolbar }
