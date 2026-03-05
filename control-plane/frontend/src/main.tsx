import React from 'react';
import ReactDOM from 'react-dom/client';
import { ClerkProvider } from '@clerk/clerk-react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';
import { App } from './app/App';
import { clerkEnabled, clerkPublishableKey } from './lib/auth';
import './styles.css';

const queryClient = new QueryClient();

const app = (
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>
);

ReactDOM.createRoot(document.getElementById('root')!).render(
  clerkEnabled ? (
    <ClerkProvider publishableKey={clerkPublishableKey}>{app}</ClerkProvider>
  ) : (
    app
  ),
);
