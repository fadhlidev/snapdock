{
  description = "Golang Dev Shell";

  inputs = {
    nixpkgs.url = "https://github.com/NixOS/nixpkgs/archive/refs/heads/nixos-25.11.tar.gz";
  };

  outputs = { self, nixpkgs }:
  let
    forAllSystems = nixpkgs.lib.genAttrs [ "x86_64-linux" "aarch64-linux" ];
  in {
    devShells = forAllSystems (system:
      let pkgs = import nixpkgs { inherit system; };
      in {
        default = pkgs.mkShell {
          packages = [ pkgs.go ];
        };
      });
  };
}
