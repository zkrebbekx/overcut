// Catches a render error in one view and shows it in place, so a bug can
// never blank the whole page.

import { Component, type ErrorInfo, type ReactNode } from "react";

interface State {
  error: Error | null;
}

export class ErrorBoundary extends Component<{ children: ReactNode; resetKey?: string }, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("overcut view error", error, info.componentStack);
  }

  componentDidUpdate(prev: { resetKey?: string }) {
    if (prev.resetKey !== this.props.resetKey && this.state.error) this.setState({ error: null });
  }

  render() {
    if (this.state.error) {
      return (
        <div role="alert" className="card border-loss/40 p-4 text-sm">
          <div className="mb-1 font-semibold text-loss">This view hit an error.</div>
          <pre className="num whitespace-pre-wrap text-xs text-ink-2">{String(this.state.error.message ?? this.state.error)}</pre>
          <button onClick={() => this.setState({ error: null })} className="chip mt-3 px-3 py-1 text-xs text-ink-2 hover:text-ink">
            Try again
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
