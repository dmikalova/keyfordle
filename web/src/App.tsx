import KeyFordle from "./pages/KeyFordle.tsx";

// The shell around the lifted page: a way back to keyforge.cards, and the
// attribution the MIT License asks for.
const App = () => (
  <div className="min-h-screen flex flex-col bg-gray-50 dark:bg-gray-900">
    <header className="max-w-4xl w-full mx-auto px-4 pt-4">
      <a
        href="https://keyforge.cards"
        className="text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white"
      >
        ← keyforge.cards
      </a>
    </header>

    <main className="flex-1">
      <KeyFordle />
    </main>

    <footer className="max-w-4xl w-full mx-auto px-4 py-6 text-center text-xs text-gray-500 dark:text-gray-400 space-y-1">
      <p>
        KeyFordle was created by the KFStats contributors (
        <a
          href="https://github.com/Valnor280/KFStats"
          className="underline hover:text-gray-900 dark:hover:text-white"
        >
          github.com/Valnor280/KFStats
        </a>
        ), MIT License.
      </p>
      <p>KeyForge is a trademark of Ghost Galaxy. Unofficial, not affiliated.</p>
    </footer>
  </div>
);

export default App;
