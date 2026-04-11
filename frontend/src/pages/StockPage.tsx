import { Navigate } from 'react-router-dom';

/** Compat: enlaces viejos a `/stock` redirigen al módulo anidado. */
export function StockPage() {
  return <Navigate to="/modules/stock/list" replace />;
}

export default StockPage;
