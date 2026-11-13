{
  description = "Versioned Smoke executable and optional GitHub runtime adapter";
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/1d060086d60cb76a40cf37109f979d88a5bd6396";
  outputs = { self, nixpkgs }:
    let
      systems = [ "aarch64-linux" "x86_64-linux" ];
      eachSystem = nixpkgs.lib.genAttrs systems;
    in {
      packages = eachSystem (system:
        let
          pkgs = import nixpkgs { inherit system; };
          smokeSource = ./.;
          operator = pkgs.writeShellApplication {
            name = "smoke-credential-runtime-adapter";
            runtimeInputs = with pkgs; [ go_1_26 git sops age python3 coreutils gnutar gnugrep ];
            text = ''
              exec ${pkgs.bash}/bin/bash ${./local/ghxd/operator.sh} "$@"
            '';
          };
          builder = pkgs.writeShellApplication {
            name = "smoke-ghxd-source-build";
            runtimeInputs = with pkgs; [ go_1_26 git coreutils nix ];
            text = ''
              set -Eeuo pipefail
              source=${smokeSource}
              root="''${HOME}/.local/state/smoke/builds"
              mkdir -p "$root/go-build" "$root/go-mod"
              export GOCACHE="$root/go-build" GOMODCACHE="$root/go-mod"
              export GOOS=linux CGO_ENABLED=0
              scratch="$(mktemp -d "$root/build.XXXXXXXX")"
              trap 'rm -rf -- "$scratch"' EXIT
              (cd "$source" && go build -mod=readonly -trimpath -buildvcs=false -ldflags=-buildid= -o "$scratch/smoke" ./cmd/smoke)
              binary="$(nix-store --add "$scratch/smoke")"
              nix-store --realise --add-root "$root/smoke-gcroot" "$binary" >/dev/null
              printf 'SMOKE_GHXD_NATIVE_ARTIFACT=%s\n' "$binary"
              printf 'SMOKE_SOURCE_BUILD=passed source=%s\n' "$source"
            '';
          };
        in {
          credential-runtime-adapter = operator;
          source-build = builder;
        });
      apps = eachSystem (system: {
        source-build = {
          type = "app";
          program = "${self.packages.${system}.source-build}/bin/smoke-ghxd-source-build";
        };
      });
    };
}
