// @ts-nocheck
'use strict';
import {padmodals} from './pad_modals';
import {colorutils} from './colorutils';
import html10n from './i18n';

const q = (selector) => document.querySelector(selector);
const qa = (selector) => Array.from(document.querySelectorAll(selector));
const getSliderBar = () => q('#ui-slider-bar');
const getSliderHandle = () => q('#ui-slider-handle');
const sliderBarWidth = () => Math.max((getSliderBar()?.getBoundingClientRect().width || 0) - 2, 0);

export const loadBroadcastSliderJS = (fireWhenAllScriptsAreLoaded) => {
  let BroadcastSlider;

  const returnLabel = q("[data-key='timeslider_returnToPad'] > a > span");
  if (returnLabel) returnLabel.textContent = html10n.get('timeslider.toolbar.returnbutton');

  (() => {
    let sliderLength = 1000;
    let sliderPos = 0;
    let sliderActive = false;
    const slidercallbacks = [];
    const savedRevisions = [];
    let sliderPlaying = false;

    const _callSliderCallbacks = (newval) => {
      sliderPos = newval;
      for (const callback of slidercallbacks) callback(newval);
    };

    const updateSliderElements = () => {
      const maxPos = sliderBarWidth();
      for (const star of savedRevisions) {
        const position = Number(star.getAttribute('data-pos') || '0');
        star.style.left = `${(position * maxPos / sliderLength) - 1}px`;
      }
      const handle = getSliderHandle();
      if (handle) handle.style.left = `${sliderPos * maxPos / sliderLength}px`;
    };

    const addSavedRevision = (position) => {
      const star = document.createElement('div');
      star.className = 'star';
      star.setAttribute('data-pos', `${position}`);
      star.style.left = `${(position * sliderBarWidth() / sliderLength) - 1}px`;
      getSliderBar()?.appendChild(star);
      star.addEventListener('mouseup', () => {
        BroadcastSlider.setSliderPosition(position);
      });
      savedRevisions.push(star);
    };

    const onSlider = (callback) => {
      slidercallbacks.push(callback);
    };

    const getSliderPosition = () => sliderPos;

    const setSliderPosition = (newpos) => {
      newpos = Number(newpos);
      if (newpos < 0 || newpos > sliderLength) return;
      if (!newpos) newpos = 0;
      window.location.hash = `#${newpos}`;

      const maxPos = sliderBarWidth();
      const handle = getSliderHandle();
      if (handle) handle.style.left = `${newpos * maxPos / sliderLength}px`;

      for (const link of qa('a.tlink')) {
        const thref = link.getAttribute('thref');
        if (!thref) continue;
        link.setAttribute('href', thref.replace('%revision%', `${newpos}`));
      }

      const revisionLabel = q('#revision_label');
      if (revisionLabel) revisionLabel.textContent = html10n.get('timeslider.version', {version: newpos});

      for (const el of qa('#leftstar, #leftstep')) el.classList.toggle('disabled', newpos === 0);
      for (const el of qa('#rightstar, #rightstep')) el.classList.toggle('disabled', newpos === sliderLength);

      sliderPos = newpos;
      _callSliderCallbacks(newpos);
    };

    const getSliderLength = () => sliderLength;

    const setSliderLength = (newlength) => {
      sliderLength = newlength;
      updateSliderElements();
    };

    const showReconnectUI = () => {
      padmodals.showModal('disconnected');
    };

    const setAuthors = (authors) => {
      const authorsList = q('#authorsList');
      if (!authorsList) return;
      authorsList.textContent = '';
      let numAnonymous = 0;
      let numNamed = 0;
      const colorsAnonymous = [];

      authors.forEach((author) => {
        if (!author) return;
        const authorColor = clientVars.colorPalette[author.colorId] || author.colorId;
        if (author.name) {
          if (numNamed !== 0) authorsList.append(', ');
          const textColor = colorutils.textColorFromBackgroundColor(authorColor, clientVars.skinName);
          const span = document.createElement('span');
          span.textContent = author.name || 'unnamed';
          span.style.backgroundColor = authorColor;
          span.style.color = textColor;
          span.className = 'author';
          authorsList.append(span);
          numNamed++;
        } else {
          numAnonymous++;
          if (authorColor) colorsAnonymous.push(authorColor);
        }
      });

      if (numAnonymous > 0) {
        const anonymousAuthorString = html10n.get('timeslider.unnamedauthors', {num: numAnonymous});
        authorsList.append(numNamed !== 0 ? ` + ${anonymousAuthorString}` : anonymousAuthorString);
        if (colorsAnonymous.length > 0) {
          authorsList.append(' (');
          colorsAnonymous.forEach((color, i) => {
            if (i > 0) authorsList.append(' ');
            const span = document.createElement('span');
            span.innerHTML = '&nbsp;';
            span.style.backgroundColor = color;
            span.className = 'author author-anonymous';
            authorsList.append(span);
          });
          authorsList.append(')');
        }
      }

      if (authors.length === 0) {
        authorsList.append(html10n.get('timeslider.toolbar.authorsList'));
      }
    };

    const playButtonUpdater = () => {
      if (!sliderPlaying) return;
      if (getSliderPosition() + 1 > sliderLength) {
        q('#playpause_button_icon')?.classList.toggle('pause');
        sliderPlaying = false;
        return;
      }
      setSliderPosition(getSliderPosition() + 1);
      setTimeout(playButtonUpdater, 100);
    };

    const playpause = () => {
      q('#playpause_button_icon')?.classList.toggle('pause');
      if (!sliderPlaying) {
        if (getSliderPosition() === sliderLength) setSliderPosition(0);
        sliderPlaying = true;
        playButtonUpdater();
      } else {
        sliderPlaying = false;
      }
    };

    BroadcastSlider = {
      onSlider,
      getSliderPosition,
      setSliderPosition,
      getSliderLength,
      setSliderLength,
      isSliderActive: () => sliderActive,
      playpause,
      addSavedRevision,
      showReconnectUI,
      setAuthors,
    };

    fireWhenAllScriptsAreLoaded.push(() => {
      document.addEventListener('keyup', (e) => {
        if (!(e instanceof KeyboardEvent)) return;
        const code = e.keyCode || e.which;
        if (code === 37) {
          q(e.shiftKey ? '#leftstar' : '#leftstep')?.dispatchEvent(new MouseEvent('click'));
        } else if (code === 39) {
          q(e.shiftKey ? '#rightstar' : '#rightstep')?.dispatchEvent(new MouseEvent('click'));
        } else if (code === 32) {
          q('#playpause_button_icon')?.dispatchEvent(new MouseEvent('click'));
        }
      });

      window.addEventListener('resize', updateSliderElements);

      getSliderBar()?.addEventListener('mousedown', (evt) => {
        const barRect = getSliderBar()?.getBoundingClientRect();
        const handle = getSliderHandle();
        if (!barRect || !handle) return;
        handle.style.left = `${evt.clientX - barRect.left}px`;
        handle.dispatchEvent(new MouseEvent('mousedown', {clientX: evt.clientX, bubbles: true}));
      });

      getSliderHandle()?.addEventListener('mousedown', (evt) => {
        const handle = getSliderHandle();
        if (!handle) return;
        const startLoc = evt.clientX;
        let currentLoc = parseInt(handle.style.left || '0');
        sliderActive = true;

        const onMove = (evt2) => {
          handle.style.pointerEvents = 'auto';
          let newloc = currentLoc + (evt2.clientX - startLoc);
          if (newloc < 0) newloc = 0;
          const maxPos = sliderBarWidth();
          if (newloc > maxPos) newloc = maxPos;
          const version = Math.floor(newloc * sliderLength / (maxPos || 1));
          const revisionLabel = q('#revision_label');
          if (revisionLabel) revisionLabel.textContent = html10n.get('timeslider.version', {version});
          handle.style.left = `${newloc}px`;
          if (getSliderPosition() !== version) _callSliderCallbacks(version);
        };

        const onUp = (evt2) => {
          document.removeEventListener('mousemove', onMove);
          document.removeEventListener('mouseup', onUp);
          sliderActive = false;
          let newloc = currentLoc + (evt2.clientX - startLoc);
          if (newloc < 0) newloc = 0;
          const maxPos = sliderBarWidth();
          if (newloc > maxPos) newloc = maxPos;
          handle.style.left = `${newloc}px`;
          setSliderPosition(Math.floor(newloc * sliderLength / (maxPos || 1)));
          if (parseInt(handle.style.left || '0') < 2) {
            handle.style.left = '2px';
          } else {
            currentLoc = parseInt(handle.style.left || '0');
          }
        };

        document.addEventListener('mousemove', onMove);
        document.addEventListener('mouseup', onUp);
      });

      q('#playpause_button_icon')?.addEventListener('click', () => {
        BroadcastSlider.playpause();
      });

      for (const stepper of qa('.stepper')) {
        stepper.addEventListener('click', () => {
          switch (stepper.id) {
            case 'leftstep':
              setSliderPosition(getSliderPosition() - 1);
              break;
            case 'rightstep':
              setSliderPosition(getSliderPosition() + 1);
              break;
            case 'leftstar': {
              let nextStar = 0;
              for (const star of savedRevisions) {
                const pos = Number(star.getAttribute('data-pos') || '0');
                if (pos < getSliderPosition() && nextStar < pos) nextStar = pos;
              }
              setSliderPosition(nextStar);
              break;
            }
            case 'rightstar': {
              let nextStar = sliderLength;
              for (const star of savedRevisions) {
                const pos = Number(star.getAttribute('data-pos') || '0');
                if (pos > getSliderPosition() && nextStar > pos) nextStar = pos;
              }
              setSliderPosition(nextStar);
              break;
            }
          }
        });
      }

      if (clientVars) {
        const wrapper = q('#timeslider-wrapper');
        if (wrapper) wrapper.style.display = '';

        if (window.location.hash.length > 1) {
          const hashRev = Number(window.location.hash.substr(1));
          if (!isNaN(hashRev)) setTimeout(() => setSliderPosition(hashRev), 1);
        }

        setSliderLength(clientVars.collab_client_vars.rev);
        setSliderPosition(clientVars.collab_client_vars.rev);
        clientVars.savedRevisions.forEach((revision) => addSavedRevision(revision.revNum, revision));
      }
    });
  })();

  BroadcastSlider.onSlider((loc) => {
    const viewLatest = q('#viewlatest');
    if (viewLatest) {
      viewLatest.textContent = `${loc === BroadcastSlider.getSliderLength() ? 'Viewing' : 'View'} latest content`;
    }
  });

  return BroadcastSlider;
};
