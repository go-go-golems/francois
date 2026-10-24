import React from "react";
import { createRoot } from "react-dom/client";
import { Provider } from "react-redux";
import { Account } from "./account";
import { Admin } from "./admin";
import { Device } from "./device";
import { Instances, Launcher, Starter } from "./instances";
import { store } from "./store";
import { Studio } from "./studio";
import { Footer, Header, Link, Notice, useRoute } from "./ui";

function Routes() {
  const { path } = useRoute();
  const parts = path.split("/").filter(Boolean);
  if (parts[0] === "instances" && parts[1] === "new") return <Starter />;
  if (parts[0] === "instances" && parts[1]) return <Launcher id={parts[1]} />;
  let page: React.ReactNode;
  if (parts[0] === "instances") page = <Instances />;
  else if (parts[0] === "account" || parts[0] === "thank-you")
    page = <Account />;
  else if (parts[0] === "admin") page = <Admin />;
  else if (parts[0] === "device") page = <Device />;
  else if (parts[0] === "studio") page = <Studio />;
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
