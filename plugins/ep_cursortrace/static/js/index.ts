/**
 * ep_cursortrace - Shows cursor/caret positions of other users in real time.
 *
 * Ported from the original ep_cursortrace Etherpad plugin.
 */

declare const pad: {
  getUserId: () => string;
  getPadId: () => string;
  collabClient: {
    sendMessage: (msg: Record<string, unknown>) => void;
    getConnectedUsers: () => Array<{
      userId: string;
      colorId: number | string;
      name?: string;
    }>;
  };
  getColorPalette: () => string[];
};

declare const $: any;

let initiated = false;
let last: [number, number] | undefined;
let globalKey = 0;

// ---------------------------------------------------------------------------
// aceEditorCSS - injects the cursortrace CSS into the ace editor iframe
// ---------------------------------------------------------------------------
export const aceEditorCSS = (): string[] => [
  'ep_cursortrace/static/css/cursortrace.css',
];

// ---------------------------------------------------------------------------
// aceInitInnerdocbodyHead - inject inner-doc CSS link into iframe head
// ---------------------------------------------------------------------------
export const aceInitInnerdocbodyHead = (
  _hookName: string,
  args: { iframeHTML: string[] },
  cb: () => void,
): void => {
  const url = '../static/plugins/ep_cursortrace/static/css/cursortrace.css';
  args.iframeHTML.push(`<link rel="stylesheet" type="text/css" href="${url}"/>`);
  cb();
};

// ---------------------------------------------------------------------------
// postAceInit - mark that the editor is ready
// ---------------------------------------------------------------------------
export const postAceInit = (): void => {
  initiated = true;
};

// ---------------------------------------------------------------------------
// Helper: convert an author ID to a safe CSS class name
// ---------------------------------------------------------------------------
const getAuthorClassName = (author: string): string | false => {
  if (!author) return false;
  const authorId = author.replace(/[^a-y0-9]/g, (c: string) => {
    if (c === '.') return '-';
    return `z${c.charCodeAt(0)}z`;
  });
  return `ep_cursortrace-${authorId}`;
};

// ---------------------------------------------------------------------------
// aceEditEvent - track local cursor position changes and broadcast them
// ---------------------------------------------------------------------------
export const aceEditEvent = (_hookName: string, args: {
  callstack: {
    editEvent: { eventType: string };
    type: string;
  };
  rep: {
    selStart: [number, number];
    selEnd: [number, number];
  };
}): void => {
  const caretMoving =
    args.callstack.editEvent.eventType === 'handleClick' ||
    args.callstack.type === 'handleKeyEvent' ||
    args.callstack.type === 'idleWorkTimer';

  if (caretMoving && initiated) {
    const Y = args.rep.selStart[0];
    const X = args.rep.selStart[1];

    if (!last || Y !== last[0] || X !== last[1]) {
      const myAuthorId = pad.getUserId();
      const padId = pad.getPadId();

      const message = {
        type: 'cursor',
        action: 'cursorPosition',
        locationY: Y,
        locationX: X,
        padId,
        myAuthorId,
      };
      last = [Y, X];

      pad.collabClient.sendMessage(message);
    }
  }
};

// ---------------------------------------------------------------------------
// Helper: truncate HTML content to `count` text characters
// ---------------------------------------------------------------------------
const htmlSubstr = (str: string, count: number): string => {
  const div = document.createElement('div');
  div.innerHTML = str;
  let remaining = count;

  const track = (el: Text): void => {
    if (remaining > 0) {
      const len = el.data.length;
      remaining -= len;
      if (remaining <= 0) {
        el.data = el.substringData(0, el.data.length + remaining);
      }
    } else {
      el.data = '';
    }
  };

  const walk = (el: Node): void => {
    let node = el.firstChild;
    if (!node) return;
    do {
      if (node.nodeType === 3) {
        track(node as Text);
      } else if (node.nodeType === 1 && node.childNodes && node.childNodes[0]) {
        walk(node);
      }
    } while ((node = node.nextSibling));
  };

  walk(div);
  return div.innerHTML;
};

// ---------------------------------------------------------------------------
// Helper: wrap every character in a span with a data-key attribute
// ---------------------------------------------------------------------------
const wrap = (target: any): string => {
  const newtarget = $('<div></div>');
  const nodes = target.contents().clone();
  if (!nodes) return '';
  nodes.each(function (this: Node) {
    if (this.nodeType === 3) {
      let newhtml = '';
      const text = (this as Text).wholeText;
      for (let i = 0; i < text.length; i++) {
        if (text[i] === ' ') {
          newhtml += `<span data-key=${globalKey}> </span>`;
        } else {
          newhtml += `<span data-key=${globalKey}>${text[i]}</span>`;
        }
        globalKey++;
      }
      newtarget.append($(newhtml));
    } else {
      $(this).html(wrap($(this)));
      newtarget.append($(this));
    }
  });
  return newtarget.html();
};

// ---------------------------------------------------------------------------
// handleClientMessage_CUSTOM - render remote users' cursor positions
// ---------------------------------------------------------------------------
export const handleClientMessage_CUSTOM = (
  _hook: string,
  context: {
    payload: {
      action: string;
      authorId: string;
      authorName: string | null;
      padId: string;
      locationX: number;
      locationY: number;
    };
  },
  cb: () => void,
): false | void => {
  const { action, authorId } = context.payload;

  // Do not render our own cursor
  if (pad.getUserId() === authorId) return false;

  const authorClass = getAuthorClassName(authorId);

  if (action === 'cursorPosition') {
    let authorName: string = context.payload.authorName ?? '';
    if (!authorName || authorName === 'null') {
      authorName = '\u{1F60A}';
    }

    // +1 because Etherpad line numbers start at 1 in the DOM
    const y = context.payload.locationY + 1;
    let x = context.payload.locationX;

    const inner = $('iframe[name="ace_outer"]').contents().find('iframe');
    let leftOffset = 0;
    if (inner.length !== 0) {
      leftOffset = parseInt($(inner).offset().left, 10);
      leftOffset += parseInt($(inner).css('padding-left'), 10);
    }

    let stickUp = false;

    // Get the target line element
    const div = $('iframe[name="ace_outer"]')
      .contents()
      .find('iframe')
      .contents()
      .find('#innerdocbody')
      .find(`div:nth-child(${y})`);

    const divWidth = div.width();

    if (div.length !== 0) {
      let top: number = $(div).offset().top;

      // Adjust for padding on the inner iframe
      top += parseInt(
        $('iframe[name="ace_outer"]').contents().find('iframe').css('paddingTop'),
        10,
      );

      const html: string = $(div).html();
      const authorWorker = `hiddenUgly${getAuthorClassName(authorId)}`;

      // If the div contains block-level elements (h1, h2, etc.) adjust x
      if ($(div).children('span').length < 1) {
        x -= 1;
      }

      // Get HTML truncated to x characters (to measure width)
      const newText = htmlSubstr(html, x);

      const newLine =
        `<span style='width:${divWidth}px' id='${authorWorker}'` +
        ` class='ghettoCursorXPos'>${newText}</span>`;

      globalKey = 0;

      // Append hidden measurement element to outer doc
      $('iframe[name="ace_outer"]')
        .contents()
        .find('#outerdocbody')
        .append(newLine);

      const worker = $('iframe[name="ace_outer"]')
        .contents()
        .find('#outerdocbody')
        .find(`#${authorWorker}`);

      // Wrap each character in a keyed span for measurement
      $(worker).html(wrap($(worker)));

      const span = $(worker).find(`[data-key="${x - 1}"]`);

      let left: number;
      if (span.length !== 0) {
        left = span.position().left;
      } else {
        left = 0;
      }

      const height: number = worker.height();
      top = top + height - (span.height() || 12);

      if (top <= 0) {
        stickUp = true;
        top += (span.height() || 12) * 2;
        if (top < 0) top = 0;
      }

      left += leftOffset;

      // Account for page-view margins
      let divMargin: string | undefined = $(div).css('margin-left');
      let innerdocbodyMargin: number = parseInt(
        $(div).parent().css('padding-left') || '0',
        10,
      );
      if (isNaN(innerdocbodyMargin)) innerdocbodyMargin = 0;

      if (divMargin) {
        const parsed = parseInt(divMargin.replace('px', ''), 10);
        if (parsed + innerdocbodyMargin > 0) {
          left += parsed;
        }
      }
      left += 18;

      // Remove the measurement element
      $('iframe[name="ace_outer"]')
        .contents()
        .find('#outerdocbody')
        .contents()
        .remove(`#${authorWorker}`);

      // Determine author color and render the caret indicator
      const users = pad.collabClient.getConnectedUsers();
      $.each(users, (_idx: number, value: { userId: string; colorId: number | string }) => {
        if (value.userId === authorId) {
          const colors: string[] = pad.getColorPalette();
          let color: string;
          if (typeof value.colorId === 'number' && colors[value.colorId]) {
            color = colors[value.colorId];
          } else {
            color = String(value.colorId);
          }

          const outBody = $('iframe[name="ace_outer"]')
            .contents()
            .find('#outerdocbody');

          // Remove any existing indicator for this author
          $('iframe[name="ace_outer"]')
            .contents()
            .find(`.caret-${authorClass}`)
            .remove();

          const location = stickUp ? 'stickUp' : 'stickDown';

          const $indicator = $(
            `<div class='caretindicator ${location} caret-${authorClass}'` +
              ` style='height:16px;left:${left}px;top:${top}px;background-color:${color}'>` +
              `<p class='stickp ${location}'></p></div>`,
          );
          $indicator.attr('title', authorName);
          $indicator.find('p').text(authorName);
          $(outBody).append($indicator);

          // Fade out after a short delay
          setTimeout(() => {
            $indicator.fadeOut(500, () => {
              $indicator.remove();
            });
          }, 2000);
        }
      });
    }
  }

  return cb();
};
