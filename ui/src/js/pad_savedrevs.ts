// @ts-nocheck
import html10n from './i18n';
import notifications from './notifications';

/**
 * Copyright 2012 Peter 'Pita' Martischka
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS-IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

let pad;

export const saveNow = () => {
  pad.collabClient.sendMessage({type: 'SAVE_REVISION'});
  notifications.add({
    title: html10n.get('pad.savedrevs.marked'),
    text: html10n.get('pad.savedrevs.timeslider') ||
        'You can view saved revisions in the timeslider',
    sticky: false,
    time: 3000,
    class_name: 'saved-revision',
  });
};

export const init = (_pad) => {
  pad = _pad;
};

export const newRevisionList = (_revisionList) => {
  // Timeslider/saved revisions UI handling is currently legacy/no-op in this frontend branch.
};
