class Rexadb < Formula
  desc "Database provisioning for developers - spin up Postgres without Docker"
  homepage "https://github.com/rexadb/rexadb"
  version "0.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.intel?
      url "https://github.com/rexadb/rexadb/releases/download/v#{version}/rexadb-darwin-amd64"
      sha256 "use actual sha256 from release"
    else
      url "https://github.com/rexadb/rexadb/releases/download/v#{version}/rexadb-darwin-arm64"
      sha256 "use actual sha256 from release"
    end
  end

  def install
    bin.install buildpath.children.first => "rexadb"
  end

  test do
    system "#{bin}/rexadb", "--version"
  end
end
