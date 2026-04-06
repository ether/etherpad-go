import notifications from 'ep_etherpad-lite/static/js/notifications';

const defaultMsg: Record<string, string> = {
  join: 'joined the pad',
  leave: 'left the pad',
};

let cssLoaded = false;
const ensureCSS = () => {
  if (cssLoaded) return;
  cssLoaded = true;
  const link = document.createElement('link');
  link.rel = 'stylesheet';
  link.href = '/static/plugins/ep_chat_log_join_leave/static/css/index.css';
  document.head.appendChild(link);
};
ensureCSS();

type ChatContext = {
  authorName: string;
  author: string;
  text: string;
  message: any;
  rendered: unknown;
  sticky: boolean;
  timestamp: number;
  timeStr: string;
  duration: number;
};

export const chatNewMessage = async (_hookName: string, context: ChatContext) => {
  const type: string | undefined = context.message?.ep_chat_log_join_leave;
  if (type == null) return;
  if (type !== 'join' && type !== 'leave') return;

  const typeId = `ep_chat_log_join_leave-${type}`;

  if (!context.authorName) context.authorName = context.author;

  // Suppress the default chat bottom popup — we show our own top-right notification instead.
  context.duration = 0;

  // Show a clean top-right notification like "Connected" or "Saved revision".
  notifications.add({
    text: `${context.authorName} ${defaultMsg[type]}`,
    sticky: false,
    time: 4000,
    position: 'top',
  });

  // Override the default chat log rendering.
  const timeElt = document.createElement('span');
  timeElt.classList.add('time');
  timeElt.append(context.timeStr);

  const nameElt = document.createElement('span');
  nameElt.classList.add('ep_chat_log_join_leave-name');
  nameElt.append(context.authorName);

  const msgElt = document.createElement('span');
  msgElt.classList.add('ep_chat_log_join_leave-message');
  msgElt.dataset.l10nId = typeId;
  msgElt.append(defaultMsg[type]);

  context.rendered = document.createElement('p');
  const rendered = context.rendered as HTMLParagraphElement;
  rendered.classList.add(typeId);
  rendered.append(timeElt, nameElt, ' ', msgElt);

  // Mimic default author class rendering.
  const authorClass = `author-${context.author.replace(/[^a-y0-9]/g, (c) => {
    if (c === '.') return '-';
    return `z${c.charCodeAt(0)}z`;
  })}`;
  rendered.classList.add(authorClass);
  rendered.dataset.authorId = context.author;
};
