'use strict';

// Alignment attribute constants
const ALIGN_ATTRIBUTE = 'align';
const ALIGNMENT_VALUES = ['left', 'center', 'right', 'justify'];

// Modul-Exports
const ep_align_module = {};

/**
 * CSS für den Ace-Editor
 */
ep_align_module.aceEditorCSS = () => ['ep_align/static/css/editor.css'];

/**
 * Registriert Block-Elemente für das Alignment
 */
ep_align_module.aceRegisterBlockElements = () => ['div'];

/**
 * Initialisiert die Alignment-Funktionalität im Ace-Editor
 */
ep_align_module.aceInitialized = (hookName, context) => {
  const editorInfo = context.editorInfo;

  // Speichere Referenz auf editorInfo für die Toolbar-Buttons
  if (typeof window !== 'undefined') {
    window.ep_align_editorInfo = editorInfo;
  }
};

/**
 * Konvertiert Alignment-Attribute zu CSS-Klassen
 */
ep_align_module.aceAttribsToClasses = (hookName, context) => {
  if (context.key === ALIGN_ATTRIBUTE && ALIGNMENT_VALUES.includes(context.value)) {
    return [`align-${context.value}`];
  }
  return [];
};

/**
 * Erstellt DOM-Elemente mit Alignment-Styling
 */
ep_align_module.aceCreateDomLine = (hookName, context) => {
  const cls = context.cls;

  for (const alignment of ALIGNMENT_VALUES) {
    if (cls.indexOf(`align-${alignment}`) !== -1) {
      return [{
        extraOpenTags: `<div style="text-align: ${alignment};">`,
        extraCloseTags: '</div>',
        cls: ''
      }];
    }
  }

  return [];
};

/**
 * Lädt die Toolbar-CSS dynamisch
 */
function loadToolbarCSS() {
  const cssUrl = '/static/plugins/ep_align/static/css/toolbar.css';

  // Prüfe, ob CSS bereits geladen
  if (document.querySelector(`link[href="${cssUrl}"]`)) return;

  const link = document.createElement('link');
  link.rel = 'stylesheet';
  link.type = 'text/css';
  link.href = cssUrl;
  document.head.appendChild(link);
}

/**
 * Erstellt die Alignment-Toolbar-Buttons
 */
function createAlignmentButtons() {
  const $ = window.jQuery || window.$;
  if (!$) return;

  const menuLeft = $('.menu_left');
  if (!menuLeft.length) return;

  // Prüfe, ob Buttons bereits existieren
  if ($('[data-key="alignLeft"]').length > 0) return;

  // Separator
  const separator = $('<li class="separator"></li>');

  // Alignment-Buttons
  const alignButtons = [
    { key: 'alignLeft', icon: 'align-left', title: 'Links ausrichten', grouped: 'left' },
    { key: 'alignCenter', icon: 'align-center', title: 'Zentrieren', grouped: 'middle' },
    { key: 'alignRight', icon: 'align-right', title: 'Rechts ausrichten', grouped: 'middle' },
    { key: 'alignJustify', icon: 'align-justify', title: 'Blocksatz', grouped: 'right' }
  ];

  const buttonElements = alignButtons.map((btn, index) => {
    const groupClass = btn.grouped === 'left' ? 'grouped-left' :
                       btn.grouped === 'right' ? 'grouped-right' : 'grouped-middle';
    return $(`
      <li data-type="button" data-key="${btn.key}">
        <a class="${groupClass}" title="${btn.title}" aria-label="${btn.title}">
          <button class="buttonicon buttonicon-${btn.icon}" title="${btn.title}" aria-label="${btn.title}">
          </button>
        </a>
      </li>
    `);
  });

  // Füge Buttons nach dem clearauthorship-Button ein
  const clearAuthButton = $('[data-key="clearauthorship"]');
  if (clearAuthButton.length) {
    clearAuthButton.after(separator);
    buttonElements.forEach((btn) => {
      separator.after(btn);
    });
  } else {
    // Fallback: Am Ende der linken Toolbar einfügen
    menuLeft.append(separator);
    buttonElements.forEach((btn) => {
      menuLeft.append(btn);
    });
  }
}

/**
 * Initialisiert Toolbar-Buttons nach dem Toolbar-Init
 */
ep_align_module.postToolbarInit = (hookName, context) => {
  const toolbar = context.toolbar;
  const $ = window.jQuery || window.$;

  // Lade die Toolbar-CSS
  loadToolbarCSS();

  // Erstelle die Alignment-Buttons
  createAlignmentButtons();

  // Registriere die Alignment-Befehle
  const alignCommands = {
    alignLeft: 'left',
    alignCenter: 'center',
    alignRight: 'right',
    alignJustify: 'justify'
  };

  Object.entries(alignCommands).forEach(([cmd, alignment]) => {
    toolbar.registerAceCommand(cmd, (cmd, ace) => {
      setAlignmentInAce(ace, alignment);
    });
  });

  // Event-Handler für die Buttons
  Object.keys(alignCommands).forEach((cmd) => {
    $(`[data-key="${cmd}"]`).on('click', () => {
      toolbar.triggerCommand(cmd);
    });
  });
};

/**
 * Setzt das Alignment im Ace-Editor
 */
function setAlignmentInAce(ace, alignment) {
  const rep = ace.ace_getRep();
  const attributeManager = ace.ace_getAttributeManager();

  if (!attributeManager) return;

  const selStart = rep.selStart;
  const selEnd = rep.selEnd;

  // Aktuelle Alignment-Einstellung ermitteln
  const currentAlignment = attributeManager.getAttributeOnLine(selStart[0], ALIGN_ATTRIBUTE);

  // Toggle: Wenn bereits gesetzt, entfernen; sonst setzen
  const newAlignment = currentAlignment === alignment ? '' : alignment;

  // Setze das Alignment für alle ausgewählten Zeilen
  for (let line = selStart[0]; line <= selEnd[0]; line++) {
    if (newAlignment) {
      attributeManager.setAttributeOnLine(line, ALIGN_ATTRIBUTE, newAlignment);
    } else {
      attributeManager.removeAttributeOnLine(line, ALIGN_ATTRIBUTE);
    }
  }
}

/**
 * Setzt das Alignment für die aktuelle Zeile (alte Methode, für Kompatibilität)
 */
function setAlignment(alignment) {
  const editorInfo = window.ep_align_editorInfo;
  if (!editorInfo || !editorInfo.ace) return;

  editorInfo.ace.callWithAce((ace) => {
    setAlignmentInAce(ace, alignment);
  }, 'setAlignment', true);
}

// Registriere das Modul global für den Plugin-Loader
if (typeof window !== 'undefined') {
  window['ep_align/static/js/index'] = ep_align_module;
}

// Für CommonJS/Node.js Kompatibilität
if (typeof module !== 'undefined' && module.exports) {
  module.exports = ep_align_module;
}

