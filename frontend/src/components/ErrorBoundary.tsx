import { Component, type ErrorInfo, type ReactNode } from 'react';

interface State { error: Error | null; }

export default class ErrorBoundary extends Component<{ children: ReactNode }, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('[ErrorBoundary]', error, info.componentStack);
  }

  render() {
    if (this.state.error) {
      return (
        <div className="error-screen">
          <div className="error-screen-card">
            <div className="error-screen-icon">⚠</div>
            <h2>Something went wrong</h2>
            <p>An unexpected error occurred. Please refresh the page to continue.</p>
            <p className="error-screen-detail">{this.state.error.message}</p>
            <button className="btn btn-primary" onClick={() => window.location.reload()}>
              Refresh the page
            </button>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}
