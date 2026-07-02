import React from 'react';

/**
 * Line-icon set. Uses currentColor, so set colour on the parent.
 *   <Icon name="search" size={16} />
 */
export function Icon({ name, size = 16, className = '', style, ...rest }) {
  const p = { width: size, height: size, fill: 'none', className, style, 'aria-hidden': true, ...rest };
  switch (name) {
    case 'search':
      return (<svg {...p} viewBox="0 0 16 16"><circle cx="7" cy="7" r="4.6" stroke="currentColor" strokeWidth="1.5" /><path d="M11 11l3.2 3.2" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" /></svg>);
    case 'bell':
      return (<svg {...p} viewBox="0 0 18 18"><path d="M5 8a4 4 0 018 0c0 2.8 1 3.8 1 3.8H4S5 10.8 5 8z" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" /><path d="M7.4 14.2a1.7 1.7 0 003.2 0" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" /></svg>);
    case 'chevron-right':
      return (<svg {...p} viewBox="0 0 16 16"><path d="M6 4l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" /></svg>);
    case 'chevron-down':
      return (<svg {...p} viewBox="0 0 16 16"><path d="M4 6l4 4 4-4" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" /></svg>);
    case 'chevron-left':
      return (<svg {...p} viewBox="0 0 16 16"><path d="M10 4L6 8l4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" /></svg>);
    case 'close':
      return (<svg {...p} viewBox="0 0 20 20"><path d="M5 5l10 10M15 5L5 15" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" /></svg>);
    case 'close-sm':
      return (<svg {...p} viewBox="0 0 16 16"><path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" /></svg>);
    case 'logout':
      return (<svg {...p} viewBox="0 0 20 20"><path d="M8 17H5a1.5 1.5 0 01-1.5-1.5v-11A1.5 1.5 0 015 3h3M13 13l3-3-3-3M16 10H8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" /></svg>);
    case 'star':
      return (<svg {...p} viewBox="0 0 20 20"><path d="M10 2.5l2.2 4.6 5 .5-3.7 3.3 1.1 4.9L10 13.7 5.4 16.3l1.1-4.9L2.8 8.1l5-.5z" fill="currentColor" /></svg>);
    case 'flag':
      return (<svg {...p} viewBox="0 0 20 20"><path d="M5 3v14M5 4h9l-2 3 2 3H5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" /></svg>);
    case 'clock':
      return (<svg {...p} viewBox="0 0 20 20"><circle cx="10" cy="10" r="7" stroke="currentColor" strokeWidth="1.5" /><path d="M10 6.5V10l2.5 1.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" /></svg>);
    case 'inbox':
      return (<svg {...p} viewBox="0 0 20 20"><path d="M3 11h4l1.4 2.2h3.2L16 11M3 11l2.4-6h9.2L17 11v4.2A1.3 1.3 0 0115.7 16.5H4.3A1.3 1.3 0 013 15.2z" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" /></svg>);
    case 'send':
      return (<svg {...p} viewBox="0 0 20 20"><path d="M17.5 3L9 11.5M17.5 3l-5.2 14-2.8-6.3L3 8z" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" /></svg>);
    case 'drafts':
      return (<svg {...p} viewBox="0 0 20 20"><path d="M12 3H5A1.3 1.3 0 003.7 4.3v11.4A1.3 1.3 0 005 17h10a1.3 1.3 0 001.3-1.3V7z M12 3v4h4" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" /></svg>);
    case 'archive':
      return (<svg {...p} viewBox="0 0 20 20"><rect x="3.2" y="4" width="13.6" height="4" rx="1" stroke="currentColor" strokeWidth="1.4" /><path d="M4.5 8v7.2A1.3 1.3 0 005.8 16.5h8.4a1.3 1.3 0 001.3-1.3V8M8.3 11h3.4" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" /></svg>);
    case 'shield':
      return (<svg {...p} viewBox="0 0 20 20"><path d="M10 3l6 2v5c0 4-3 6-6 7-3-1-6-3-6-7V5z" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" /></svg>);
    case 'trash':
      return (<svg {...p} viewBox="0 0 20 20"><path d="M4 6h12M8 6V4.2h4V6M6 6l.8 10.2h6.4L14 6" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" /></svg>);
    case 'folder':
      return (<svg {...p} viewBox="0 0 18 18"><path d="M2 5.2A1.4 1.4 0 013.4 3.8H7l1.4 1.4h6.2A1.4 1.4 0 0116 6.6v6.2a1.4 1.4 0 01-1.4 1.4H3.4A1.4 1.4 0 012 12.8z" stroke="currentColor" strokeWidth="1.3" /></svg>);
    case 'arrow-right':
      return (<svg {...p} viewBox="0 0 16 16"><path d="M3 8h9M9 5l3 3-3 3" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" /></svg>);
    case 'reply':
      return (<svg {...p} viewBox="0 0 18 18"><path d="M8 4L3 9l5 5M3 9h8a4 4 0 014 4v1" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" /></svg>);
    case 'forward':
      return (<svg {...p} viewBox="0 0 18 18"><path d="M10 4l5 5-5 5M15 9H7a4 4 0 00-4 4v1" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" /></svg>);
    case 'plus':
      return (<svg {...p} viewBox="0 0 14 14"><path d="M7 3v8M3 7h8" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" /></svg>);
    case 'check':
      return (<svg {...p} viewBox="0 0 14 14"><path d="M3 7.4l2.6 2.6L11 4.6" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" /></svg>);
    default:
      return null;
  }
}
