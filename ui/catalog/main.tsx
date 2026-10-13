import React from "react";
import { createRoot } from "react-dom/client";
import { Provider } from "react-redux";
import { Account } from "./account";
import { Admin } from "./admin";
import { AppDetail, Catalog } from "./catalog";
import { Instances, Launcher } from "./instances";
import { store } from "./store";
import { Footer, Header, Link, Notice, useRoute } from "./ui";

function Routes() {
  const { path } = useRoute();
  const parts = path.split("/").filter(Boolean);
  if (parts[0] === "instances" && parts[1]) return <Launcher id={parts[1]} />;
  let page: React.ReactNode;
  if (path === "/") page = <Catalog />;
  else if (parts[0] === "apps" && parts[1]) page = <AppDetail id={parts[1]} />;
  else if (parts[0] === "instances") page = <Instances />;
  else if (parts[0] === "account") page = <Account />;
  else if (parts[0] === "admin") page = <Admin />;
  else
    page = (
      <main className="lost">
        <h1>Nothing here.</h1>
        <Link className="u" to="/">
          Back to the catalog
        </Link>
      </main>
    );
  return (
    <>
      <Header />
      <div className="page">
        {page}
        <Footer />
      </div>
    </>
  );
}

createRoot(document.getElementById("root")!).render(
  <Provider store={store}>
    <Routes />
    <Notice />
  </Provider>,
);
