import { Navigate } from 'react-router-dom';

export default function ProtectedRoute({ children }: { children: React.ReactNode }) {
  if (!localStorage.getItem('mcp_token')) return <Navigate to="/login" replace />;
  return <>{children}</>;
}