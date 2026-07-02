import React from 'react';
import { ACCOUNT, NAV, FOLDERS, WEBMAIL_LABELS, EMAILS, initials } from '../data/sample.js';
import { AppShell } from '../components/organisms/AppShell.jsx';
import { PageHeader } from '../components/molecules/PageHeader.jsx';
import { Webmail as WebmailPane } from '../components/organisms/Webmail.jsx';
import { Button } from '../components/atoms/Button.jsx';

const account = { ...ACCOUNT, initials: initials(ACCOUNT.name) };

/** Webmail page — full-width app shell wrapping the three-pane client. */
export function Webmail(props) {
  return (
    <AppShell nav={NAV} current="webmail" account={account} wide {...props}>
      <PageHeader
        title="Webmail"
        sub="jamie@acme.io · SOGo"
        actions={<Button variant="primary">Compose</Button>}
      />
      <WebmailPane folders={FOLDERS} labels={WEBMAIL_LABELS} emails={EMAILS} selected={0} account={account} />
    </AppShell>
  );
}
