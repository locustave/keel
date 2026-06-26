# This file is managed automatically by the keel release workflow.
# Do not edit by hand — changes will be overwritten on the next release.
# Source: https://github.com/locustave/keel/.github/workflows/release.yml

class Keel < Formula
  desc "Controlled execution governance for AI coding agents"
  homepage "https://github.com/locustave/keel"
  url "https://github.com/locustave/keel/archive/refs/tags/v0.0.0.tar.gz"
  sha256 "PLACEHOLDER_SHA256_UPDATED_BY_RELEASE_WORKFLOW"
  license "MIT"
  head "https://github.com/locustave/keel.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/keel"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/keel --version 2>&1", 1)
    system bin/"keel", "--help"
  end
end
