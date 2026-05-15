/**
 * Etherpad UI Web Components
 *
 * Import this module to register all custom elements.
 * Most components come from the etherpad-webcomponents npm package.
 * EpPluginToolbar is local-only (not in the npm package).
 */

import 'etherpad-webcomponents/EpNotification.js';
import 'etherpad-webcomponents/EpModal.js';
import 'etherpad-webcomponents/EpToast.js';
import 'etherpad-webcomponents/EpColorPicker.js';
import 'etherpad-webcomponents/EpDropdown.js';
import 'etherpad-webcomponents/EpToolbarSelect.js';
import 'etherpad-webcomponents/EpCheckbox.js';
import './EpPluginToolbar';

export {EpNotification} from 'etherpad-webcomponents';
export {EpModal} from 'etherpad-webcomponents';
export {EpToastContainer} from 'etherpad-webcomponents';
export {EpColorPicker} from 'etherpad-webcomponents';
export {EpDropdown, EpDropdownItem} from 'etherpad-webcomponents';
export {EpPluginToolbar} from './EpPluginToolbar';
export {EpToolbarSelect} from 'etherpad-webcomponents';
