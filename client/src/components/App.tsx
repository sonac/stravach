import TelegramLoginButton, { TelegramUser } from "./TelegramAuth";
import "./App.css";

function onTelegramAuth(user: any) {
  // Create the payload to be sent to the server
  const payload = {
    user: {
      id: user.id,
      first_name: user.first_name,
      last_name: user.last_name,
      username: user.username,
    },
  };

  // Send the payload to the /tg-auth endpoint
  fetch("/tg-auth", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
    credentials: "include",
  })
    .then((response) => {
      if (response.ok) {
        return response.text();
      } else {
        throw new Error("Failed to authenticate");
      }
    })
    .then((data) => {
      console.log("Authentication successful:", data);
      window.location.href = "/user/" + user.id;
    })
    .catch((error) => {
      console.error("Error:", error);
      alert("Authentication failed. Please try again.");
    });
}

function App() {
  return (
    <>
      <div>
        <h1>Welcome to the Strava Activity Renamer Bot!</h1>
        <p>
          This bot helps you automatically rename your Strava activities using
          AI suggestions.
        </p>
        <p>
          Simply authorize your Strava account, and our bot will take care of
          the rest!
        </p>
        <TelegramLoginButton
          botName="strava_snitch_bot"
          dataOnauth={(user: TelegramUser) => onTelegramAuth(user)}
        />
      </div>
    </>
  );
}

export default App;
