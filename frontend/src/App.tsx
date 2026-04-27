import { Navigate, Route, Routes } from 'react-router-dom';
import ProtectedRoute from './components/ProtectedRoute';
import Layout from './components/Layout';
import LoginPage from './pages/LoginPage';
import RegisterPage from './pages/RegisterPage';
import RoomsPage from './pages/RoomsPage';
import ChatPage from './pages/ChatPage';
import DMsPage from './pages/DMsPage';
import AccountPage from './pages/AccountPage';

export default function App() {
  return (
    <Routes>
      <Route path="/login"    element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />

      <Route element={<ProtectedRoute />}>
        <Route element={<Layout />}>
          <Route index element={<Navigate to="/rooms" replace />} />
          <Route path="/rooms"     element={<RoomsPage />} />
          <Route path="/rooms/:id" element={<ChatPage type="room" />} />
          <Route path="/dms"       element={<DMsPage />} />
          <Route path="/dms/:id"   element={<ChatPage type="dm" />} />
          <Route path="/account"   element={<AccountPage />} />
        </Route>
      </Route>

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
