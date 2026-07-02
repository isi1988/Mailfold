import React from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';

// Design system — import order matters: tokens -> base -> components.
import './ds/styles/tokens.css';
import './ds/styles/base.css';
import './ds/styles/components.css';

import { I18nProvider } from './i18n/index.jsx';
import { AuthProvider } from './auth/AuthContext.jsx';
import { App } from './App.jsx';

createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <BrowserRouter>
      <I18nProvider>
        <AuthProvider>
          <App />
        </AuthProvider>
      </I18nProvider>
    </BrowserRouter>
  </React.StrictMode>,
);
