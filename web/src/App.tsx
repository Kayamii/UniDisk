import { Navigate, Route, Routes } from "react-router-dom";
import { useAuth } from "./lib/auth";
import { AppLayout } from "./components/AppLayout";
import { LoginPage } from "./pages/LoginPage";
import { ChangePasswordPage } from "./pages/ChangePasswordPage";
import { FilesPage } from "./pages/FilesPage";
import { PoolPage } from "./pages/PoolPage";
import { ProvidersPage } from "./pages/ProvidersPage";
import { UsersPage } from "./pages/UsersPage";
import { RolesPage } from "./pages/RolesPage";
import { ApiKeysPage } from "./pages/ApiKeysPage";

export default function App() {
  const { user, loading, can } = useAuth();

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center text-muted-foreground">
        Loading…
      </div>
    );
  }

  if (!user) {
    return <LoginPage />;
  }

  // Force the password change before anything else is reachable.
  if (user.must_change_password) {
    return <ChangePasswordPage forced />;
  }

  return (
    <AppLayout>
      <Routes>
        {can("files.view") && <Route path="/" element={<FilesPage />} />}
        {can("files.view") && <Route path="/pool" element={<PoolPage />} />}
        {can("files.view") && <Route path="/keys" element={<ApiKeysPage />} />}
        {can("providers.manage") && <Route path="/providers" element={<ProvidersPage />} />}
        {can("users.manage") && <Route path="/users" element={<UsersPage />} />}
        {can("roles.manage") && <Route path="/roles" element={<RolesPage />} />}
        <Route path="*" element={<Navigate to={can("files.view") ? "/" : "/account"} replace />} />
        <Route path="/account" element={<ChangePasswordPage />} />
      </Routes>
    </AppLayout>
  );
}
