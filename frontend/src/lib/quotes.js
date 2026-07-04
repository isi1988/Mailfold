// Collapses deep quoted-reply history in a message body so a long back-and-forth
// thread shows only the most recent 3 quoted messages inline, with anything
// older tucked behind a "show N earlier" toggle. Two independent heuristics are
// used since a message body can be either HTML or plain text:
//
//  - HTML: virtually every mail client (Gmail, Outlook, Apple Mail, Yandex, and
//    Mailfold's own reply prefixing further down) wraps each quoted message in
//    a <blockquote>, nested one level per hop. The 4th (and every deeper)
//    blockquote in document order is wrapped in a native <details>/<summary> —
//    which toggles without JavaScript, so it still works inside the fully
//    sandboxed (sandbox="") iframe the reader renders untrusted HTML in.
//  - Plain text: quoting conventionally prefixes each line with one "> " per
//    level of nesting (exactly what Mailfold's own reply() does, and what most
//    other clients emit too), so counting leading ">" runs per line gives the
//    same depth signal without any HTML to walk.

/**
 * collapseQuotedHtml transforms an HTML message body, wrapping the 4th+
 * blockquote (in document order) in a collapsed <details>. Returns the
 * original html unchanged when there are 3 or fewer blockquotes.
 *   showLabel(hiddenCount) => string — the <summary> text to show
 */
export function collapseQuotedHtml(html, showLabel) {
  if (!html) return html;
  let doc;
  try {
    doc = new DOMParser().parseFromString(html, 'text/html');
  } catch {
    return html;
  }
  const blockquotes = Array.from(doc.querySelectorAll('blockquote'));
  if (blockquotes.length <= 3) return html;

  const cutoff = blockquotes[3];
  const hiddenCount = blockquotes.length - 3;
  const details = doc.createElement('details');
  const summary = doc.createElement('summary');
  summary.textContent = showLabel(hiddenCount);
  details.appendChild(summary);
  if (cutoff.parentNode) {
    cutoff.parentNode.insertBefore(details, cutoff);
    details.appendChild(cutoff);
  }
  return doc.body ? doc.body.innerHTML : html;
}

// quoteDepth counts the leading ">" runs (each optionally followed by one
// space) at the start of a plain-text line — its quoting nesting level.
function quoteDepth(line) {
  let i = 0;
  let depth = 0;
  while (line[i] === '>') {
    depth++;
    i++;
    if (line[i] === ' ') i++;
  }
  return depth;
}

/**
 * collapseQuotedText splits a plain-text body into a visible head and a
 * hidden tail once the quoting depth exceeds 3, returning
 *   { visible, hidden, hiddenCount }
 * hiddenCount is 0 (and hidden is '') when nothing needs collapsing.
 */
export function collapseQuotedText(text) {
  if (!text) return { visible: text || '', hidden: '', hiddenCount: 0 };
  const lines = text.split('\n');
  const depths = lines.map(quoteDepth);
  const maxDepth = depths.reduce((m, d) => Math.max(m, d), 0);
  if (maxDepth <= 3) return { visible: text, hidden: '', hiddenCount: 0 };

  const cutIndex = depths.findIndex(d => d > 3);
  return {
    visible: lines.slice(0, cutIndex).join('\n'),
    hidden: lines.slice(cutIndex).join('\n'),
    hiddenCount: maxDepth - 3,
  };
}
